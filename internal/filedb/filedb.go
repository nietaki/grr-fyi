package filedb

// 2026-02-26 22:48:06.057510+00:001 3gp
// 2026-02-26 22:48:06.057541+00:0011 DAT
// 2026-02-26 22:48:06.057549+00:001 DII
// 2026-02-26 22:48:06.057555+00:001 LFP
// 2026-02-26 22:48:06.057561+00:001 LST
// 2026-02-26 22:48:06.057579+00:0011 OPT
// 2026-02-26 22:48:06.057585+00:005 amr
// 2026-02-26 22:48:06.057592+00:001 avi
// 2026-02-26 22:48:06.057598+00:004 csv
// 2026-02-26 22:48:06.057604+00:0020 m4a
// 2026-02-26 22:48:06.057610+00:0029 m4v
// 2026-02-26 22:48:06.057616+00:001 md
// 2026-02-26 22:48:06.057626+00:00154 mov
// 2026-02-26 22:48:06.057632+00:002 mp3
// 2026-02-26 22:48:06.057638+00:001068 mp4
// 2026-02-26 22:48:06.057644+00:0016 opus
// 2026-02-26 22:48:06.057649+00:00849657 pdf
// 2026-02-26 22:48:06.057655+00:001 pluginpayloadattachment
// 2026-02-26 22:48:06.057664+00:001 txt
// 2026-02-26 22:48:06.057670+00:005 wav
// 2026-02-26 22:48:06.057676+00:002 xls
// 2026-02-26 22:48:06.057681+00:0010 xlsx
// 2026-02-26 22:48:06.057687+00:001 zip

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
var allFilenames []string
var filenamesByType map[string][]string

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
	`CREATE INDEX idx_documents_filetype ON documents(filetype);`,
	`CREATE INDEX idx_documents_dataset ON documents(dataset);`,
	`CREATE INDEX idx_documents_filesize ON documents(filesize);`,
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
	// stmt.()
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
		re := regexp.MustCompile(`\s+`)
		contents = re.ReplaceAllString(contents, " ")
	}

	stmt.SetText("$textContents", contents)
	stmt.SetInt64("$textLength", int64(len(contents)))

	err = zero(stmt)

	if err != nil {
		fmt.Printf("Error inserting document: %v\n", err)
		return err
	}

	// TODO
	return nil
}

func Query(filetype string, datasets []string, text string) []string {
	// TODO
	return []string{}
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

func getRandom(filenames []string) string {
	ret := lo.Sample(filenames)

	if ret == "" {
		return "not_found.png"
	}
	return ret
}

func StoreFilenames(f []string) {
	allFilenames = f
	filenamesByType = lo.GroupBy(allFilenames, FileType)
}

func FileCount() int {
	conn, err := dbPool.Take(context.TODO())
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

func GetRandomFilename() string {
	// get a random int in the range of 0 to a billion
	rnd := rand.Intn(1000000000)
	targetSalt := math.Sin(float64(rnd))

	conn, err := dbPool.Take(context.TODO())
	if err != nil {
		panic(err)
	}
	defer dbPool.Put(conn)

	stmt := conn.Prep("SELECT path FROM documents WHERE salt >= $target ORDER BY salt ASC LIMIT 1;")
	stmt.SetFloat("$target", targetSalt)

	filename, err := one(stmt, func(s *sqlite.Stmt) string {
		return s.GetText("path")
	})
	if err != nil {
		fmt.Printf("Error getting random filename: %v\n", err)
		return "not_found.png"
	}

	return filename
}

func GetRandomFilenameByTypes(filetypes []string) string {
	if len(filetypes) == 0 {
		return getRandom(allFilenames)
	}

	countForFiletypes := lo.SumBy(filetypes, func(filetype string) int {
		return len(filenamesByType[filetype])
	})

	if countForFiletypes == 0 {
		return "not_found.png"
	}

	// get random integer in the range
	randomIndex := rand.Intn(countForFiletypes)

	for _, filetype := range filetypes {
		filenames := filenamesByType[filetype]
		if randomIndex < len(filenames) {
			return filenames[randomIndex]
		}
		randomIndex -= len(filenames)
	}
	return "not_found.png"
}
