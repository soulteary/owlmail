package main

import (
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal filename",
			input:    "test.txt",
			expected: "test.txt",
		},
		{
			name:     "filename with slash",
			input:    "path/to/file.txt",
			expected: "path_to_file.txt",
		},
		{
			name:     "filename with backslash",
			input:    "path\\to\\file.txt",
			expected: "path_to_file.txt",
		},
		{
			name:     "filename with colon",
			input:    "file:name.txt",
			expected: "file_name.txt",
		},
		{
			name:     "filename with asterisk",
			input:    "file*name.txt",
			expected: "file_name.txt",
		},
		{
			name:     "filename with question mark",
			input:    "file?name.txt",
			expected: "file_name.txt",
		},
		{
			name:     "filename with quotes",
			input:    "file\"name.txt",
			expected: "file_name.txt",
		},
		{
			name:     "filename with angle brackets",
			input:    "file<name>txt",
			expected: "file_name_txt",
		},
		{
			name:     "filename with pipe",
			input:    "file|name.txt",
			expected: "file_name.txt",
		},
		{
			name:     "filename with multiple invalid chars",
			input:    "file/name*test?file.txt",
			expected: "file_name_test_file.txt",
		},
		{
			name:  "very long filename",
			input: "a" + string(make([]byte, 200)),
			expected: func() string {
				s := "a" + string(make([]byte, 200))
				if len(s) > 100 {
					return s[:100]
				}
				return s
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if tt.name == "very long filename" {
				if len(result) > 100 {
					t.Errorf("Expected filename length <= 100, got %d", len(result))
				}
			} else if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
