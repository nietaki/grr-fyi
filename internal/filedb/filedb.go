package filedb

// 1 3gp
// 11 DAT
// 1 DII
// 1 LFP
// 1 LST
// 11 OPT
// 5 amr
// 1 avi
// 4 csv
// 20 m4a
// 29 m4v
// 1 md
// 154 mov
// 2 mp3
// 1068 mp4
// 16 opus
// 849657 pdf
// 1 pluginpayloadattachment
// 1 txt
// 5 wav
// 2 xls
// 10 xlsx
// 1 zip

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path"
	"regexp"
	"strings"

	lo "github.com/samber/lo"
	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitemigration"
	"zombiezen.com/go/sqlite/sqlitex"
)

var dbPool *sqlitex.Pool

const dbPath = "./db/filedb.sqlite"

var migrations []string = []string{
	`CREATE TABLE documents (
    id INTEGER PRIMARY KEY,
    dataset TEXT,
    path TEXT,
    extension TEXT,
    filetype TEXT,
    filesize INTEGER,
    text_contents TEXT,
    text_length INTEGER,
    salt REAL
  );
  `,
	`CREATE INDEX idx_documents_filetype ON documents(filetype, salt);`,
	`CREATE INDEX idx_documents_dataset ON documents(dataset, salt);`,
	// `CREATE INDEX idx_documents_filesize ON documents(filesize);`,
	`CREATE INDEX idx_documents_salt ON documents(salt);`,
	`CREATE VIRTUAL TABLE documents_fts USING fts5(text_contents, tokenize='trigram case_sensitive 0 remove_diacritics 1', content='documents', content_rowid='id');`,
	`CREATE TRIGGER documents_ai AFTER INSERT ON documents BEGIN
    INSERT INTO documents_fts(rowid, text_contents) VALUES (new.id, new.text_contents);
  END;`,
}

func migrate() {
	os.Remove(dbPath)
	schema := sqlitemigration.Schema{
		Migrations: migrations,
	}
	pool := sqlitemigration.NewPool(dbPath, schema, sqlitemigration.Options{
		Flags: sqlite.OpenReadWrite | sqlite.OpenCreate,
		OnError: func(err error) {
			panic(err)
		},
	})
	// Get a connection. This blocks until the migration completes.
	conn, err := pool.Get(context.TODO())
	if err != nil {
		// handle error
	}
	defer pool.Put(conn)
}

func setupPool() {
	// https://pkg.go.dev/zombiezen.com/go/sqlite#example-package-Http
	poolOptions := sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite,
		PoolSize: 10, // Optional: set maximum number of open connections
	}
	var err error
	dbPool, err = sqlitex.NewPool(dbPath, poolOptions)
	if err != nil {
		panic(err)
	}
}

type DocumentRecord struct {
	Dataset      string
	Path         string
	Extension    string
	Filetype     string
	Filesize     int64
	TextContents string
	TextLength   int
}

type QueryError struct {
	Message string
}

func (e *QueryError) Error() string {
	return e.Message
}

func NewQueryError(message string) *QueryError {
	return &QueryError{Message: message}
}

func Init() {
	migrate()
	setupPool()
}

func one[T any](stmt *sqlite.Stmt, extractor func(*sqlite.Stmt) T) (T, error) {
	var defaultValue T
	hasRow, err := stmt.Step()
	if err != nil {
		fmt.Printf("Error executing a one() query: %v\n", err)
		return defaultValue, err
	}
	if !hasRow {
		return defaultValue, NewQueryError("no rows found")
	}
	// will work even if there's multiple rows
	defer stmt.Reset()

	return extractor(stmt), nil
}

func zero(stmt *sqlite.Stmt) error {
	hasRow, err := stmt.Step()
	if err != nil {
		fmt.Printf("Error executing a zero() query: %v\n", err)
		return err
	}
	if hasRow {
		return NewQueryError("expected zero rows, but got at least one")
	}
	return nil
}

var spaceRegex *regexp.Regexp = regexp.MustCompile(`\s+`)

func AddDocument(originalPath string) error {
	// TODO context
	conn, err := dbPool.Take(context.TODO())
	defer dbPool.Put(conn)
	if err != nil {
		panic(err)
	}

	textContentsPath := strings.Replace(originalPath, "raw_data/", "processed_data/", 1)
	textContentsPath = textContentsPath + ".content.txt"

	stmt := conn.Prep("INSERT INTO documents (dataset, path, extension, filetype, filesize, text_contents, text_length, salt) VALUES ($dataset, $path, $extension, $filetype, $filesize, $textContents, $textLength, sin(random()))")

	dataset := strings.Split(originalPath, "/")[1]
	stmt.SetText("$dataset", dataset)
	stmt.SetText("$path", originalPath)
	ext := path.Ext(originalPath)
	stmt.SetText("$extension", ext)
	filetype := FileType(originalPath)
	stmt.SetText("$filetype", filetype)
	fileInfo, err := os.Stat(originalPath)
	if err != nil {
		fmt.Printf("Error getting file info for %s: %v\n", originalPath, err)
		return err
	}
	stmt.SetInt64("$filesize", fileInfo.Size())

	var contents string
	byteContents, err := os.ReadFile(textContentsPath)
	if err != nil {
		contents = ""
	} else {
		contents = string(byteContents)
		// clean up the contents
		contents = spaceRegex.ReplaceAllString(contents, " ")
	}

	stmt.SetText("$textContents", contents)
	stmt.SetInt64("$textLength", int64(len(contents)))

	err = zero(stmt)

	if err != nil {
		fmt.Printf("Error inserting document: %v\n", err)
		return err
	}

	return nil
}

func FileType(filename string) string {
	ext := path.Ext(filename)
	ext = strings.ToLower(ext)
	switch ext {
	case ".pdf":
		return "pdf"
	case ".3gp", ".avi", ".m4v", ".mov", ".mp4":
		return "video"
	case ".amr", ".m4a", ".mp3", ".opus", ".wav":
		return "audio"
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".svg":
		return "image"
	default:
		return "other"
	}
}

func AllFiletypes() []string {
	return []string{"pdf", "video", "audio", "image", "other"}
}

func FileCount(ctx context.Context) int {
	conn, err := dbPool.Take(ctx)
	if err != nil {
		return -1
	}
	defer dbPool.Put(conn)

	stmt := conn.Prep("SELECT COUNT(*) as ct FROM documents;")
	ct, err := one(stmt, func(s *sqlite.Stmt) int {
		return int(s.GetInt64("ct"))
	})
	if err != nil {
		fmt.Printf("Error counting documents: %v\n", err)
		return -1
	}
	return ct
}

func getRandomSalt() float64 {
	rnd := rand.Intn(1000000000)
	return math.Sin(float64(rnd))
}

// func GetRandomFilename(ctx context.Context) string {
// 	conn, err := dbPool.Take(ctx)
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer dbPool.Put(conn)

// 	stmt := conn.Prep("SELECT path FROM documents ORDER BY abs(salt - $target) ASC LIMIT 1;")
// 	stmt.SetFloat("$target", getRandomSalt())

// 	filename, err := one(stmt, func(s *sqlite.Stmt) string {
// 		return s.GetText("path")
// 	})
// 	if err != nil {
// 		fmt.Printf("Error getting random filename: %v\n", err)
// 		return "not_found.png"
// 	}

// 	return filename
// }

func GetRandomFilenameByQuery(ctx context.Context, query string) string {
	conn, err := dbPool.Take(ctx)
	if err != nil {
		panic(err)
	}
	defer dbPool.Put(conn)

	stmt := conn.Prep(`
    SELECT path 
    FROM documents 
    INNER JOIN documents_fts ON documents.id = documents_fts.rowid 
    WHERE documents_fts MATCH ? 
    ORDER BY abs(salt - ?) ASC LIMIT 1;`)
	stmt.BindText(1, query)
	stmt.BindFloat(2, getRandomSalt())

	filename, err := one(stmt, func(s *sqlite.Stmt) string {
		return s.GetText("path")
	})
	if err != nil {
		fmt.Printf("Error getting random filename by contents: %v\n", err)
		return "not_found.png"
	}

	return filename
}

func GetRandomFilenameByTypes(ctx context.Context, filetypes []string) string {
	conn, err := dbPool.Take(ctx)
	if err != nil {
		panic(err)
	}

	if len(filetypes) == 0 {
		filetypes = AllFiletypes()
	}
	defer dbPool.Put(conn)

	questionMarks := lo.Map(filetypes, func(_ string, _ int) string { return "?" })
	placeholders := strings.Join(questionMarks, ", ")

	query := fmt.Sprintf("SELECT path FROM documents WHERE filetype IN (%s) AND salt >= ? ORDER BY salt ASC LIMIT 1;", placeholders)

	// fmt.Printf("Query: %s\n", query)

	stmt := conn.Prep(query)

	idx := 1

	for _, filetype := range filetypes {
		stmt.BindText(idx, filetype)
		idx++
	}

	stmt.BindFloat(idx, getRandomSalt())

	filename, err := one(stmt, func(s *sqlite.Stmt) string {
		return s.GetText("path")
	})
	if err != nil {
		fmt.Printf("Error getting random filename by types: %v\n", err)
		return "not_found.png"
	}
	return filename
}
