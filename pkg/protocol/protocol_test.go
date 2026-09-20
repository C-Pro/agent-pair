package protocol

import (
	"strings"
	"testing"
)

func TestFormatTurn(t *testing.T) {
	tests := []struct {
		name      string
		sender    string
		recipient string
		msg       string
		wantStart string
		wantEnd   string
	}{
		{
			name:      "plain message",
			sender:    "gemini",
			recipient: "muse",
			msg:       "Hello world",
			wantStart: "[ CQ gemini -> muse ]\n\nHello world",
			wantEnd:   "[ gemini over ]",
		},
		{
			name:      "already has CQ prefix",
			sender:    "gemini",
			recipient: "muse",
			msg:       "[ CQ custom -> other ]\nSome message",
			wantStart: "[ CQ custom -> other ]\nSome message",
			wantEnd:   "[ gemini over ]",
		},
		{
			name:      "already has over suffix",
			sender:    "gemini",
			recipient: "muse",
			msg:       "Some message\n[ gemini over ]",
			wantStart: "[ CQ gemini -> muse ]\n\nSome message\n[ gemini over ]",
			wantEnd:   "[ gemini over ]",
		},
		{
			name:      "already has out suffix",
			sender:    "gemini",
			recipient: "muse",
			msg:       "Goodbye\n[ gemini out ]",
			wantStart: "[ CQ gemini -> muse ]\n\nGoodbye\n[ gemini out ]",
			wantEnd:   "[ gemini out ]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := FormatTurn(tt.sender, tt.recipient, tt.msg)
			if !strings.HasPrefix(formatted, tt.wantStart) {
				t.Errorf("FormatTurn() got start %q, want prefix %q", formatted, tt.wantStart)
			}
			if !strings.HasSuffix(formatted, tt.wantEnd) {
				t.Errorf("FormatTurn() got end %q, want suffix %q", formatted, tt.wantEnd)
			}
		})
	}
}

func TestFormatBootstrapPrompt(t *testing.T) {
	t.Run("read only true", func(t *testing.T) {
		prompt := FormatBootstrapPrompt("gemini", "muse", "/test/dir", true)
		if !strings.Contains(prompt, "READ-ONLY permissions") {
			t.Errorf("expected read-only warning in prompt")
		}
		if !strings.Contains(prompt, "Your callsign: muse") {
			t.Errorf("expected follower callsign")
		}
		if !strings.Contains(prompt, "Your partner (Lead Agent): gemini") {
			t.Errorf("expected leader callsign")
		}
		if !strings.Contains(prompt, "Working Directory: /test/dir") {
			t.Errorf("expected working directory")
		}
	})

	t.Run("read only false", func(t *testing.T) {
		prompt := FormatBootstrapPrompt("gemini", "muse", "/test/dir", false)
		if strings.Contains(prompt, "READ-ONLY permissions") {
			t.Errorf("did not expect read-only warning when readOnly is false")
		}
	})
}

func TestExtractLatestTurn(t *testing.T) {
	t.Run("strict pattern", func(t *testing.T) {
		screen := `
  ┃  Some previous UI junk
  ┃  [ CQ gemini -> muse ]
  ┃  What is your plan?
  ┃  [ gemini over ]
  ┃
  ┃  [ CQ muse -> gemini ]
  ┃  I suggest we refactor the mux interface first.
  ┃  Here is the plan:
  ┃  1. Define PaneHandle
  ┃  2. Implement tmux
  ┃  [ muse over ]
  ┃
  • OpenCode 1.18.30
`
		body, found := ExtractLatestTurn(screen, "muse", "gemini")
		if !found {
			t.Fatalf("expected to find turn, but found nothing")
		}
		expected := "I suggest we refactor the mux interface first.\nHere is the plan:\n1. Define PaneHandle\n2. Implement tmux"
		if body != expected {
			t.Fatalf("unexpected turn body:\nGot: %q\nWant: %q", body, expected)
		}
	})

	t.Run("strict pattern with out", func(t *testing.T) {
		screen := `
[ CQ muse -> gemini ]
Session complete. Goodbye!
[ muse out ]
`
		body, found := ExtractLatestTurn(screen, "muse", "gemini")
		if !found {
			t.Fatalf("expected to find turn with out")
		}
		if body != "Session complete. Goodbye!" {
			t.Fatalf("got %q, want %q", body, "Session complete. Goodbye!")
		}
	})

	t.Run("relaxed pattern without brackets", func(t *testing.T) {
		screen := `
CQ muse -> gemini
Here is a relaxed response without brackets.
muse over
`
		body, found := ExtractLatestTurn(screen, "muse", "gemini")
		if !found {
			t.Fatalf("expected to find relaxed turn")
		}
		if body != "Here is a relaxed response without brackets." {
			t.Fatalf("got %q", body)
		}
	})

	t.Run("fallback pattern before sender over", func(t *testing.T) {
		screen := `
Here is some output from agent.
The task is finished.
muse over
`
		body, found := ExtractLatestTurn(screen, "muse", "gemini")
		if !found {
			t.Fatalf("expected to find fallback turn")
		}
		if !strings.Contains(body, "The task is finished.") {
			t.Fatalf("got %q", body)
		}
	})

	t.Run("fallback pattern with prompt delimiter", func(t *testing.T) {
		screen := `
User -> muse
Previous context
-> muse
Here is the actual answer
[ muse over ]
`
		body, found := ExtractLatestTurn(screen, "muse", "gemini")
		if !found {
			t.Fatalf("expected to find fallback with delimiter")
		}
		if !strings.Contains(body, "Here is the actual answer") {
			t.Fatalf("got %q", body)
		}
	})

	t.Run("no turn found", func(t *testing.T) {
		screen := "Just random logs without any turn markers"
		body, found := ExtractLatestTurn(screen, "muse", "gemini")
		if found {
			t.Fatalf("expected no turn found, got %q", body)
		}
	})
}

func TestHasTurnFinished(t *testing.T) {
	tests := []struct {
		name     string
		screen   string
		sender   string
		expected bool
	}{
		{
			name:     "bracketed over",
			screen:   "[ CQ muse -> gemini ]\nDone with turn.\n[ muse over ]\n",
			sender:   "muse",
			expected: true,
		},
		{
			name:     "bracketed out",
			screen:   "[ CQ muse -> gemini ]\nSigning off.\n[ muse out ]\n",
			sender:   "muse",
			expected: true,
		},
		{
			name:     "unbracketed over word boundary",
			screen:   "muse over",
			sender:   "muse",
			expected: true,
		},
		{
			name:     "unbracketed out word boundary",
			screen:   "muse out",
			sender:   "muse",
			expected: true,
		},
		{
			name:     "case insensitive",
			screen:   "[ MUSE OVER ]",
			sender:   "muse",
			expected: true,
		},
		{
			name:     "not finished - ongoing",
			screen:   "[ CQ muse -> gemini ]\nStill thinking...",
			sender:   "muse",
			expected: false,
		},
		{
			name:     "different sender finished",
			screen:   "[ gemini over ]",
			sender:   "muse",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasTurnFinished(tt.screen, tt.sender)
			if got != tt.expected {
				t.Errorf("HasTurnFinished() = %v, want %v", got, tt.expected)
			}
		})
	}
}
