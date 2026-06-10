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
	"log/slog"
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

var dbPath = "./db/filedb.sqlite"

func SetDbPath(path string) {
	dbPath = path
}

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

func migrate(ctx context.Context) error {
	os.Remove(dbPath)
	schema := sqlitemigration.Schema{
		Migrations: migrations,
	}
	pool := sqlitemigration.NewPool(dbPath, schema, sqlitemigration.Options{
		Flags: sqlite.OpenReadWrite | sqlite.OpenCreate,
		OnError: func(err error) {
			panic(err) // this one is ok
		},
	})
	// Get a connection. This blocks until the migration completes.
	conn, err := pool.Get(ctx)
	defer pool.Put(conn)
	return err
}

func setupPool() error {
	// https://pkg.go.dev/zombiezen.com/go/sqlite#example-package-Http
	poolOptions := sqlitex.PoolOptions{
		Flags:    sqlite.OpenReadWrite,
		PoolSize: 10, // Optional: set maximum number of open connections
	}
	var err error
	dbPool, err = sqlitex.NewPool(dbPath, poolOptions)
	return err
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

func Init(ctx context.Context) error {
	err := migrate(ctx)
	if err != nil {
		return err
	}
	return setupPool()
}

func one[T any](stmt *sqlite.Stmt, extractor func(*sqlite.Stmt) T) (T, error) {
	var defaultValue T
	hasRow, err := stmt.Step()
	defer stmt.Reset()
	if err != nil {
		slog.Error("error executing a one() query", "error", err)
		return defaultValue, err
	}
	if !hasRow {
		return defaultValue, NewQueryError("no rows found")
	}

	return extractor(stmt), nil
}

func zero(stmt *sqlite.Stmt) error {
	hasRow, err := stmt.Step()
	if err != nil {
		slog.Error("error executing a zero() query", "error", err)
		return err
	}
	if hasRow {
		return NewQueryError("expected zero rows, but got at least one")
	}
	return nil
}

var spaceRegex *regexp.Regexp = regexp.MustCompile(`\s+`)

func AddDocument(ctx context.Context, originalPath string) error {
	conn, err := dbPool.Take(ctx)
	defer dbPool.Put(conn)
	if err != nil {
		return err
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
		slog.Error("error getting file info", "path", originalPath, "error", err)
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
		slog.Error("error inserting document", "error", err)
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

func FileCount(ctx context.Context) (int, error) {
	conn, err := dbPool.Take(ctx)
	defer dbPool.Put(conn)
	if err != nil {
		return -1, err
	}

	stmt := conn.Prep("SELECT COUNT(*) as ct FROM documents;")
	ct, err := one(stmt, func(s *sqlite.Stmt) int {
		return int(s.GetInt64("ct"))
	})
	if err != nil {
		slog.Error("error counting documents", "error", err)
		return -1, err
	}
	return ct, nil
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

func GetRandomFilenameByQuery(ctx context.Context, query string) (string, error) {
	conn, err := dbPool.Take(ctx)
	defer dbPool.Put(conn)
	if err != nil {
		return "not_found.png", err
	}

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
		slog.Error("error getting random filename by contents", "error", err)
		return "not_found.png", nil
	}

	return filename, nil
}

func GetRandomFilenameByTypes(ctx context.Context, filetypes []string) (string, error) {
	conn, err := dbPool.Take(ctx)
	defer dbPool.Put(conn)
	if err != nil {
		return "not_found.png", err
	}

	if len(filetypes) == 0 {
		filetypes = AllFiletypes()
	}

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
		slog.Error("error getting random filename by types", "error", err)
		return "not_found.png", err
	}
	return filename, nil
}
