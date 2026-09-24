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
		turnID    string
		wantStart string
		wantEnd   string
	}{
		{
			name:      "plain message",
			sender:    "gemini",
			recipient: "muse",
			msg:       "Hello world",
			turnID:    "a1b2",
			wantStart: "[ CQ gemini -> muse #a1b2 ]\n\nHello world",
			wantEnd:   "[ gemini over #a1b2 ]",
		},
		{
			name:      "quoted envelope is wrapped, not reused",
			sender:    "gemini",
			recipient: "muse",
			msg:       "You said:\n[ CQ muse -> gemini #0001 ]\nold reply\n[ muse over #0001 ]",
			turnID:    "beef",
			wantStart: "[ CQ gemini -> muse #beef ]\n\nYou said:",
			wantEnd:   "[ gemini over #beef ]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted := FormatTurn(tt.sender, tt.recipient, tt.msg, tt.turnID)
			if !strings.HasPrefix(formatted, tt.wantStart) {
				t.Errorf("FormatTurn() got start %q, want prefix %q", formatted, tt.wantStart)
			}
			if !strings.HasSuffix(formatted, tt.wantEnd) {
				t.Errorf("FormatTurn() got end %q, want suffix %q", formatted, tt.wantEnd)
			}
		})
	}
}

func TestNewTurnIDIsDistinct(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		seen[NewTurnID()] = true
	}
	if len(seen) < 8 {
		t.Fatalf("NewTurnID() produced only %d distinct ids in 64 draws", len(seen))
	}
}

func TestFormatBootstrapPrompt(t *testing.T) {
	t.Run("read only true", func(t *testing.T) {
		prompt := FormatBootstrapPrompt("gemini", "muse", "/test/dir", "a1b2", true)
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
		if !strings.Contains(prompt, "This turn's id is a1b2.") {
			t.Errorf("expected the turn id to be stated in the prompt")
		}
		if !strings.Contains(prompt, "linting, type checking, compiling, or running the test suite") {
			t.Errorf("expected the follower to be told it may request benign verification from the lead")
		}
	})

	t.Run("read only false", func(t *testing.T) {
		prompt := FormatBootstrapPrompt("gemini", "muse", "/test/dir", "a1b2", false)
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
		body, found := ExtractLatestTurn(screen, "muse", "gemini", "")
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
		body, found := ExtractLatestTurn(screen, "muse", "gemini", "")
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
		body, found := ExtractLatestTurn(screen, "muse", "gemini", "")
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
		body, found := ExtractLatestTurn(screen, "muse", "gemini", "")
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
		body, found := ExtractLatestTurn(screen, "muse", "gemini", "")
		if !found {
			t.Fatalf("expected to find fallback with delimiter")
		}
		if !strings.Contains(body, "Here is the actual answer") {
			t.Fatalf("got %q", body)
		}
	})

	t.Run("no turn found", func(t *testing.T) {
		screen := "Just random logs without any turn markers"
		body, found := ExtractLatestTurn(screen, "muse", "gemini", "")
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
		{
			// Regression: a bare word-boundary match treated ordinary prose as
			// an end-of-turn marker and cut the reply short.
			name:     "prose mentioning the callsign is not a marker",
			screen:   "For this job I would pick muse over codex, because it reads faster.",
			sender:   "muse",
			expected: false,
		},
		{
			name:     "marker carrying a turn id",
			screen:   "[ muse over #a1b2 ]",
			sender:   "muse",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasTurnFinished(tt.screen, tt.sender, "")
			if got != tt.expected {
				t.Errorf("HasTurnFinished() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCountTurnEndMarkers(t *testing.T) {
	screen := "[ muse over ]\nold output\n[ muse out ]\n[ codex over ]"
	if got := CountTurnEndMarkers(screen, "muse", ""); got != 2 {
		t.Fatalf("CountTurnEndMarkers() = %d, want 2", got)
	}
}

func TestBootstrapDoesNotEchoConcreteFollowerMarkers(t *testing.T) {
	prompt := FormatBootstrapPrompt("codex", "gemini-3.8-flash", "/tmp/repo", "a1b2", true)
	if CountTurnEndMarkers(prompt, "gemini-3.8-flash", "") != 0 {
		t.Fatal("bootstrap prompt contains a concrete follower completion marker")
	}
}

func TestIsTurnComplete(t *testing.T) {
	// The pane shows the leader's prompt echoed back, then the follower's reply.
	sent := FormatTurn("codex", "muse", "Review the diff.", "beef")

	t.Run("id tagged reply completes the turn", func(t *testing.T) {
		screen := sent + "\n[ CQ muse -> codex #beef ]\nLooks fine.\n[ muse over #beef ]\n"
		if !IsTurnComplete(screen, "muse", "codex", "beef") {
			t.Fatal("expected the id-tagged reply to complete the turn")
		}
	})

	t.Run("untagged reply after the prompt echo completes the turn", func(t *testing.T) {
		screen := sent + "\n[ CQ muse -> codex ]\nLooks fine.\n[ muse over ]\n"
		if !IsTurnComplete(screen, "muse", "codex", "beef") {
			t.Fatal("expected an untagged reply inside the window to complete the turn")
		}
	})

	t.Run("prompt echo alone does not complete the turn", func(t *testing.T) {
		if IsTurnComplete(sent, "muse", "codex", "beef") {
			t.Fatal("the echoed prompt completed the turn on its own")
		}
	})

	t.Run("quoted earlier reply does not complete the turn", func(t *testing.T) {
		// Regression: quoting the follower's previous reply back to it put a
		// completion marker on screen after the watermark, so wait() returned
		// immediately with stale content.
		quoted := FormatTurn("codex", "muse",
			"You previously said:\n[ CQ muse -> codex #0001 ]\nUse a mutex.\n[ muse over #0001 ]\nDoes that still hold?",
			"beef")
		if IsTurnComplete(quoted, "muse", "codex", "beef") {
			t.Fatal("a marker quoted inside the prompt completed the turn")
		}
	})

	t.Run("stale scrollback does not complete the turn", func(t *testing.T) {
		screen := "[ CQ muse -> codex #0001 ]\nOld answer.\n[ muse over #0001 ]\n" + sent
		if IsTurnComplete(screen, "muse", "codex", "beef") {
			t.Fatal("a marker left in scrollback completed the turn")
		}
	})

	t.Run("prose is not a marker", func(t *testing.T) {
		screen := sent + "\nI would pick muse over codex here, still working.\n"
		if IsTurnComplete(screen, "muse", "codex", "beef") {
			t.Fatal("prose mentioning the callsign completed the turn")
		}
	})

	t.Run("without an id and without the echo nothing completes", func(t *testing.T) {
		if IsTurnComplete("[ muse over ]", "muse", "codex", "") {
			t.Fatal("an unanchored, untagged marker completed the turn")
		}
	})
}

func TestExtractLatestTurnPrefersWindowOverQuotedReply(t *testing.T) {
	sent := FormatTurn("codex", "muse",
		"Earlier you said:\n[ CQ muse -> codex #0001 ]\nUse a mutex.\n[ muse over #0001 ]",
		"beef")
	screen := sent + "\n[ CQ muse -> codex #beef ]\nUse a channel instead.\n[ muse over #beef ]\n"

	body, found := ExtractLatestTurn(screen, "muse", "codex", "beef")
	if !found {
		t.Fatal("expected to find the reply")
	}
	if body != "Use a channel instead." {
		t.Fatalf("got %q, want the current reply rather than the quoted one", body)
	}
}

func TestReplyStartVisible(t *testing.T) {
	sent := FormatTurn("codex", "muse", "Review the diff.", "beef")

	t.Run("reply header on screen", func(t *testing.T) {
		screen := "[ CQ muse -> codex #beef ]\nA. First.\nB. Second.\n[ muse over #beef ]\n"
		if !ReplyStartVisible(screen, "muse", "codex", "beef") {
			t.Fatal("expected the id-tagged header to mark the reply as whole")
		}
	})

	t.Run("prompt echo on screen", func(t *testing.T) {
		// A follower that drops the id from its header is still whole when the
		// leader's own prompt is above it.
		screen := sent + "\nLooks fine.\n[ muse over ]\n"
		if !ReplyStartVisible(screen, "muse", "codex", "beef") {
			t.Fatal("expected the prompt echo to mark the reply as whole")
		}
	})

	t.Run("beginning scrolled out of the capture", func(t *testing.T) {
		// Regression: a follower on the alternate screen kept only the rows its
		// pane showed, so wait printed sections E-F of a reply as the reply.
		screen := "E. Risks.\nF. Questions.\n[ muse over #beef ]\n"
		if ReplyStartVisible(screen, "muse", "codex", "beef") {
			t.Fatal("a reply without its header or the prompt echo was taken as whole")
		}
	})

	t.Run("header of another turn does not count", func(t *testing.T) {
		screen := "[ CQ muse -> codex #0001 ]\nOld.\n[ muse over #0001 ]\nF. Questions.\n[ muse over #beef ]\n"
		if ReplyStartVisible(screen, "muse", "codex", "beef") {
			t.Fatal("an earlier turn's header was taken as this reply's beginning")
		}
	})

	t.Run("no turn id", func(t *testing.T) {
		if !ReplyStartVisible("F. Questions.\n[ muse over ]\n", "muse", "codex", "") {
			t.Fatal("without a turn id the reply should be assumed whole")
		}
	})
}
