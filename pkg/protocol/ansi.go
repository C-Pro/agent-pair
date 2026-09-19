package protocol

import (
	"regexp"
	"strings"
)

var (
	// ansiRegex matches ANSI escape sequences (colors, cursor movements, etc.)
	ansiRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\([a-zA-Z]|\x1b\][^\a\x1b]*(\a|\x1b\\)`)
)

// StripANSI removes ANSI escape sequences from text.
func StripANSI(input string) string {
	return ansiRegex.ReplaceAllString(input, "")
}

// CleanTUIArtifacts removes box-drawing artifacts and normalize lines.
func CleanTUIArtifacts(input string) string {
	cleaned := StripANSI(input)
	// Replace carriage returns
	cleaned = strings.ReplaceAll(cleaned, "\r\n", "\n")
	cleaned = strings.ReplaceAll(cleaned, "\r", "\n")

	lines := strings.Split(cleaned, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Remove leading vertical bar artifacts from opencode/tui
		for strings.HasPrefix(trimmed, "┃") || strings.HasPrefix(trimmed, "│") || strings.HasPrefix(trimmed, "╹") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(trimmed, "┃"), "│"), "╹"))
		}
		result = append(result, trimmed)
	}

	return strings.Join(result, "\n")
}
