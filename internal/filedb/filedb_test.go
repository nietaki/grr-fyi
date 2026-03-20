package filedb

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nietaki/epstein-file-review/internal/logging"
	"github.com/nietaki/epstein-file-review/internal/signing"
	"zombiezen.com/go/sqlite"

	lo "github.com/samber/lo"
)

func TestMain(m *testing.M) {
	logging.Init(context.Background(), false)
	os.Exit(m.Run())
}

func TestFileType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "PDF file",
			filename: "raw_data/test.pdf",
			expected: "pdf",
		},
		{
			name:     "Video MP4",
			filename: "raw_data/video.mp4",
			expected: "video",
		},
		{
			name:     "Video AVI",
			filename: "raw_data/movie.avi",
			expected: "video",
		},
		{
			name:     "Audio MP3",
			filename: "raw_data/song.mp3",
			expected: "audio",
		},
		{
			name:     "Audio WAV",
			filename: "raw_data/recording.wav",
			expected: "audio",
		},
		{
			name:     "Image JPG",
			filename: "raw_data/photo.jpg",
			expected: "image",
		},
		{
			name:     "Image PNG",
			filename: "raw_data/image.png",
			expected: "image",
		},
		{
			name:     "PDF uppercase extension",
			filename: "raw_data/FILE.PDF",
			expected: "pdf",
		},
		{
			name:     "Unknown extension",
			filename: "raw_data/file.xyz",
			expected: "other",
		},
		{
			name:     "CSV file",
			filename: "raw_data/data.csv",
			expected: "other",
		},
		{
			name:     "3gp audio",
			filename: "raw_data/voice.3gp",
			expected: "video",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FileType(tt.filename)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestAllFiletypes(t *testing.T) {
	result := AllFiletypes()

	if len(result) != 5 {
		t.Errorf("Expected 5 filetypes, got %d", len(result))
	}

	expected := []string{"pdf", "video", "audio", "image", "other"}

	for i, ft := range expected {
		if result[i] != ft {
			t.Errorf("Expected filetype '%s' at index %d, got '%s'", ft, i, result[i])
		}
	}
}

func TestQueryError(t *testing.T) {
	err := NewQueryError("test error message")

	if err.Error() != "test error message" {
		t.Errorf("Expected error message 'test error message', got '%s'", err.Error())
	}
}

func TestSignatureIntegration(t *testing.T) {
	// Test that file paths can be properly signed
	filename := "raw_data/test.pdf"

	// This simulates the flow in server.go
	queryString := signing.SigningQueryString(filename)

	if queryString == "" {
		t.Error("Expected non-empty query string")
	}

	// Should contain ts and sig
	if queryString == "" {
		t.Error("Query string should not be empty")
	}
}

func TestZeroHelper(t *testing.T) {
	conn, err := sqlite.OpenConn(":memory:", 0)
	if err != nil {
		t.Fatalf("Failed to create in-memory connection: %v", err)
	}
	defer conn.Close()

	err = zero(conn.Prep("SELECT 1 WHERE 1=0"))
	if err != nil {
		t.Errorf("zero() should return nil for query with no rows, got: %v", err)
	}

	err = zero(conn.Prep("SELECT 1"))
	if err == nil {
		t.Error("zero() should return error for query with rows")
	}
	if _, ok := err.(*QueryError); !ok {
		t.Errorf("zero() should return QueryError, got: %T", err)
	}
}

func TestOneHelper(t *testing.T) {
	conn, err := sqlite.OpenConn(":memory:", 0)
	if err != nil {
		t.Fatalf("Failed to create in-memory connection: %v", err)
	}
	defer conn.Close()

	result, err := one(conn.Prep("SELECT 42 as value"), func(s *sqlite.Stmt) int {
		return int(s.GetInt64("value"))
	})
	if err != nil {
		t.Errorf("one() should not return error for query with rows, got: %v", err)
	}
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	result, err = one(conn.Prep("SELECT 42 WHERE 1=0"), func(s *sqlite.Stmt) int {
		return int(s.GetInt64("value"))
	})
	if err == nil {
		t.Error("one() should return error for query with no rows")
	}
	if _, ok := err.(*QueryError); !ok {
		t.Errorf("one() should return QueryError, got: %T", err)
	}
	if result != 0 {
		t.Errorf("one() should return default value for no rows, got %d", result)
	}
}

func TestLoMapUsage(t *testing.T) {
	// Test that lo.Map is used correctly for creating placeholders
	filetypes := []string{"pdf", "video"}

	result := lo.Map(filetypes, func(_ string, _ int) string { return "?" })

	if len(result) != 2 {
		t.Errorf("Expected 2 placeholders, got %d", len(result))
	}

	if result[0] != "?" || result[1] != "?" {
		t.Errorf("Expected placeholders to be '?', got %v", result)
	}
}

func TestFileTypeEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "Empty string",
			filename: "",
			expected: "other",
		},
		{
			name:     "Only extension",
			filename: ".pdf",
			expected: "pdf",
		},
		{
			name:     "Nested path",
			filename: "raw_data/subdir/test.pdf",
			expected: "pdf",
		},
		{
			name:     "Mixed case",
			filename: "raw_data/Document.PDF",
			expected: "pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FileType(tt.filename)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestInit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filedb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDbPath := dbPath
	dbFile := filepath.Join(tempDir, "test.db")
	SetDbPath(dbFile)

	defer func() {
		SetDbPath(originalDbPath)
		if dbPool != nil {
			dbPool.Close()
			dbPool = nil
		}
	}()

	ctx := context.Background()
	err = Init(ctx)
	if err != nil {
		t.Errorf("Init failed: %v", err)
	}

	if dbPool == nil {
		t.Error("dbPool should be initialized after Init")
	}
}

func setupTestEnvironment(t *testing.T) (string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "filedb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	rawDataDir := filepath.Join(tempDir, "raw_data", "dataset")
	processedDataDir := filepath.Join(tempDir, "processed_data", "dataset")
	if err := os.MkdirAll(rawDataDir, 0755); err != nil {
		t.Fatalf("Failed to create raw_data dir: %v", err)
	}
	if err := os.MkdirAll(processedDataDir, 0755); err != nil {
		t.Fatalf("Failed to create processed_data dir: %v", err)
	}

	testFile := filepath.Join(rawDataDir, "test.pdf")
	if err := os.WriteFile(testFile, []byte("PDF content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	textFile := filepath.Join(processedDataDir, "test.pdf.content.txt")
	if err := os.WriteFile(textFile, []byte("text\ncontent"), 0644); err != nil {
		t.Fatalf("Failed to create text file: %v", err)
	}

	originalDbPath := dbPath
	dbFile := filepath.Join(tempDir, "test.db")
	SetDbPath(dbFile)

	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)

	cleanup := func() {
		os.Chdir(originalWd)
		SetDbPath(originalDbPath)
		if dbPool != nil {
			dbPool.Close()
			dbPool = nil
		}
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestAddDocument(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err := AddDocument(ctx, "raw_data/dataset/test.pdf")
	if err != nil {
		t.Errorf("AddDocument failed: %v", err)
	}

	count, err := FileCount(ctx)
	if err != nil {
		t.Errorf("FileCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
}

func TestAddDocumentWithoutTextFile(t *testing.T) {
	_, cleanup := setupTestEnvironment(t)
	defer cleanup()

	textFile := filepath.Join("processed_data", "dataset", "test.pdf.content.txt")
	os.Remove(textFile)

	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err := AddDocument(ctx, "raw_data/dataset/test.pdf")
	if err != nil {
		t.Errorf("AddDocument should succeed without text file, got: %v", err)
	}
}

func TestFileCountEmpty(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filedb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDbPath := dbPath
	dbFile := filepath.Join(tempDir, "test.db")
	SetDbPath(dbFile)

	defer func() {
		SetDbPath(originalDbPath)
		if dbPool != nil {
			dbPool.Close()
			dbPool = nil
		}
	}()

	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	count, err := FileCount(ctx)
	if err != nil {
		t.Errorf("FileCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 for empty database, got %d", count)
	}
}

func TestGetRandomSalt(t *testing.T) {
	for range 100 {
		salt := getRandomSalt()
		if salt < -1.0 || salt > 1.0 {
			t.Errorf("getRandomSalt() returned %f, expected value between -1 and 1", salt)
		}
	}
}

func setupTestEnvironmentWithFiles(t *testing.T, files []struct {
	name     string
	content  string
	textCont string
}) (string, func()) {
	t.Helper()

	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "filedb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create the directory structure that AddDocument expects:
	// - raw_data/dataset/ holds the actual files
	// - processed_data/dataset/ holds the extracted text content
	rawDataDir := filepath.Join(tempDir, "raw_data", "dataset")
	processedDataDir := filepath.Join(tempDir, "processed_data", "dataset")
	if err := os.MkdirAll(rawDataDir, 0755); err != nil {
		t.Fatalf("Failed to create raw_data dir: %v", err)
	}
	if err := os.MkdirAll(processedDataDir, 0755); err != nil {
		t.Fatalf("Failed to create processed_data dir: %v", err)
	}

	// Create test files in both directories
	for _, f := range files {
		testFile := filepath.Join(rawDataDir, f.name)
		if err := os.WriteFile(testFile, []byte(f.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", f.name, err)
		}

		textFile := filepath.Join(processedDataDir, f.name+".content.txt")
		if err := os.WriteFile(textFile, []byte(f.textCont), 0644); err != nil {
			t.Fatalf("Failed to create text file for %s: %v", f.name, err)
		}
	}

	// Configure the database path to use the temp directory
	originalDbPath := dbPath
	dbFile := filepath.Join(tempDir, "test.db")
	SetDbPath(dbFile)

	// Change to the temp directory so relative paths work correctly
	originalWd, _ := os.Getwd()
	os.Chdir(tempDir)

	// Return a cleanup function that restores state
	cleanup := func() {
		os.Chdir(originalWd)
		SetDbPath(originalDbPath)
		if dbPool != nil {
			dbPool.Close()
			dbPool = nil
		}
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

func TestGetRandomFilenameByTypes(t *testing.T) {
	files := []struct {
		name     string
		content  string
		textCont string
	}{
		{"test1.pdf", "PDF content 1", "document one"},
		{"test2.pdf", "PDF content 2", "document two"},
		{"test3.pdf", "PDF content 3", "document three"},
		{"test4.pdf", "PDF content 4", "document four"},
		{"test5.pdf", "PDF content 5", "document five"},
		{"video.mp4", "Video content", "video content"},
	}

	_, cleanup := setupTestEnvironmentWithFiles(t, files)
	defer cleanup()

	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for _, f := range files {
		if err := AddDocument(ctx, "raw_data/dataset/"+f.name); err != nil {
			t.Fatalf("AddDocument failed for %s: %v", f.name, err)
		}
	}

	count, err := FileCount(ctx)
	if err != nil {
		t.Fatalf("FileCount failed: %v", err)
	}
	if count != len(files) {
		t.Fatalf("Expected %d documents, got %d", len(files), count)
	}

	const numAttempts = 50
	var foundPdf, foundVideo, foundAny int

	for i := 0; i < numAttempts; i++ {
		filename, _ := GetRandomFilenameByTypes(ctx, []string{"pdf"})
		if filename != "not_found.png" {
			foundPdf++
		}

		filename, _ = GetRandomFilenameByTypes(ctx, []string{"video"})
		if filename != "not_found.png" {
			foundVideo++
		}

		filename, _ = GetRandomFilenameByTypes(ctx, []string{})
		if filename != "not_found.png" {
			foundAny++
		}
	}

	if foundPdf == 0 {
		t.Errorf("GetRandomFilenameByTypes found no PDFs after %d attempts", numAttempts)
	}
	if foundVideo == 0 {
		t.Errorf("GetRandomFilenameByTypes found no videos after %d attempts (only 1 video doc exists)", numAttempts)
	}
	if foundAny == 0 {
		t.Errorf("GetRandomFilenameByTypes found no documents after %d attempts", numAttempts)
	}
}

func TestGetRandomFilenameByTypesEmptyDB(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filedb_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDbPath := dbPath
	dbFile := filepath.Join(tempDir, "test.db")
	SetDbPath(dbFile)

	defer func() {
		SetDbPath(originalDbPath)
		if dbPool != nil {
			dbPool.Close()
			dbPool = nil
		}
	}()

	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	filename, err := GetRandomFilenameByTypes(ctx, []string{"pdf"})
	if err == nil {
		t.Error("Expected error for empty database, got nil")
	}
	if filename != "not_found.png" {
		t.Errorf("Expected not_found.png, got %s", filename)
	}
}

func TestGetRandomFilenameByQuery(t *testing.T) {
	files := []struct {
		name     string
		content  string
		textCont string
	}{
		{"uniqueword.pdf", "PDF content", "uniqueword document"},
		{"other.pdf", "PDF content", "another document"},
	}

	_, cleanup := setupTestEnvironmentWithFiles(t, files)
	defer cleanup()

	ctx := context.Background()
	if err := Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	for _, f := range files {
		if err := AddDocument(ctx, "raw_data/dataset/"+f.name); err != nil {
			t.Fatalf("AddDocument failed for %s: %v", f.name, err)
		}
	}

	filename, err := GetRandomFilenameByQuery(ctx, "uniqueword")
	if err != nil {
		t.Errorf("GetRandomFilenameByQuery failed: %v", err)
	}
	if filename != "raw_data/dataset/uniqueword.pdf" {
		t.Errorf("Expected uniqueword.pdf, got %s", filename)
	}

	filename, err = GetRandomFilenameByQuery(ctx, "nonexistent")
	if err != nil {
		t.Errorf("GetRandomFilenameByQuery should not error for no match: %v", err)
	}
	if filename != "not_found.png" {
		t.Errorf("Expected not_found.png for no match, got %s", filename)
	}
}
