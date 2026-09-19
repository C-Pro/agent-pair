package agent

import (
	"testing"
)

func TestDeriveCallsign(t *testing.T) {
	tests := []struct {
		model    string
		fallback string
		expected string
	}{
		{
			model:    "opencode/muse-spark-1.3-contributor-free",
			fallback: "muse",
			expected: "muse-spark-1.3",
		},
		{
			model:    "opencode/big-pickle",
			fallback: "opencode",
			expected: "big-pickle",
		},
		{
			model:    "Lemonade/Qwen3.6-35B-A3B-MTP-GGUF",
			fallback: "qwen",
			expected: "qwen3.6-35b-a3b",
		},
		{
			model:    "ollama/gemma4:26b",
			fallback: "gemma",
			expected: "gemma4-26b",
		},
		{
			model:    "claude-3-7-sonnet-latest",
			fallback: "claude",
			expected: "claude-3-7-sonnet",
		},
		{
			model:    "gemini-3.8-flash",
			fallback: "gemini",
			expected: "gemini-3.8-flash",
		},
		{
			model:    "o3-mini",
			fallback: "codex",
			expected: "o3-mini",
		},
		{
			model:    "",
			fallback: "gemini",
			expected: "gemini",
		},
	}

	for _, tt := range tests {
		got := DeriveCallsign(tt.model, tt.fallback)
		if got != tt.expected {
			t.Errorf("DeriveCallsign(%q, %q) = %q; want %q", tt.model, tt.fallback, got, tt.expected)
		}
	}
}
