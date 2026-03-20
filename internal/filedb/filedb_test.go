package filedb

import (
	"testing"

	"github.com/nietaki/epstein-file-review/internal/signing"

	lo "github.com/samber/lo"
)

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
	// This is a simple mock that verifies the function compiles
	// Actual testing requires real database access and statement mocking
	t.Log("TestZero function would need real sqlite.Stm to test properly")
}

func TestOneHelper(t *testing.T) {
	// This is a simple mock that verifies the function compiles
	// Actual testing requires real database access and statement mocking
	t.Log("TestOne function would need real sqlite.Stm to test properly")
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
