package protocol

import (
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "color sequences",
			input:    "\x1b[31mred\x1b[0m and \x1b[32mgreen\x1b[0m",
			expected: "red and green",
		},
		{
			name:     "cursor movement",
			input:    "\x1b[2J\x1b[Hclear screen",
			expected: "clear screen",
		},
		{
			name:     "OSC title sequence with bel",
			input:    "\x1b]0;My Window Title\x07content",
			expected: "content",
		},
		{
			name:     "OSC sequence with ST",
			input:    "\x1b]0;Title\x1b\\content",
			expected: "content",
		},
		{
			name:     "character set selection",
			input:    "\x1b(Bstandard",
			expected: "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.input)
			if got != tt.expected {
				t.Errorf("StripANSI(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCleanTUIArtifacts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "carriage returns normalized",
			input:    "line 1\r\nline 2\rline 3",
			expected: "line 1\nline 2\nline 3",
		},
		{
			name:     "opencode box-drawing vertical bars",
			input:    "┃ first line\n│ second line\n╹ third line\n┃   nested line",
			expected: "first line\nsecond line\nthird line\nnested line",
		},
		{
			name:     "multiple consecutive bar artifacts",
			input:    "┃┃ nested bars\n││ line",
			expected: "nested bars\nline",
		},
		{
			name:     "mixed ANSI and bars",
			input:    "\x1b[34m┃\x1b[0m \x1b[1mclean content\x1b[0m",
			expected: "clean content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanTUIArtifacts(tt.input)
			if got != tt.expected {
				t.Errorf("CleanTUIArtifacts(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func FuzzCleanTUIArtifacts(f *testing.F) {
	f.Add("normal string")
	f.Add("\x1b[31mcolor\x1b[0m")
	f.Add("┃ test\r\n│ line 2")
	f.Fuzz(func(t *testing.T, orig string) {
		cleaned := CleanTUIArtifacts(orig)
		for _, b := range []byte(cleaned) {
			if b == '\r' {
				t.Errorf("cleaned string contains carriage return: %q", cleaned)
			}
		}
	})
}
