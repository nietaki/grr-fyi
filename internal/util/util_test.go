package util

import "testing"

func TestHumanizeMemory(t *testing.T) {
	// Test boundary values
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{
			name:     "Zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "1 byte",
			bytes:    1,
			expected: "1 B",
		},
		{
			name:     "1023 bytes",
			bytes:    1023,
			expected: "1023 B",
		},
		{
			name:     "1024 bytes (1 KB)",
			bytes:    1024,
			expected: "1.00 KB",
		},
		{
			name:     "1025 bytes",
			bytes:    1025,
			expected: "1.00 KB",
		},
		{
			name:     "1048576 bytes (1 MB)",
			bytes:    1048576,
			expected: "1.00 MB",
		},
		{
			name:     "1073741824 bytes (1 GB)",
			bytes:    1073741824,
			expected: "1.00 GB",
		},
		{
			name:     "Very large number",
			bytes:    1099511627776,
			expected: "1024.00 GB",
		},
		{
			name:     "2.5 KB",
			bytes:    2560,
			expected: "2.50 KB",
		},
		{
			name:     "3.75 MB",
			bytes:    3932160,
			expected: "3.75 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HumanizeMemory(tt.bytes)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
