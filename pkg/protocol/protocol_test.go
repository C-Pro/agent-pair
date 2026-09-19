package protocol

import (
	"testing"
)

func TestFormatTurn(t *testing.T) {
	msg := "Hello world"
	formatted := FormatTurn("gemini", "muse", msg)

	expectedStart := "[ CQ gemini -> muse ]\n\nHello world"
	expectedEnd := "[ gemini over ]"

	if formatted[:len(expectedStart)] != expectedStart {
		t.Fatalf("unexpected start: %q", formatted)
	}
	if formatted[len(formatted)-len(expectedEnd):] != expectedEnd {
		t.Fatalf("unexpected end: %q", formatted)
	}
}

func TestExtractLatestTurn(t *testing.T) {
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
}

func TestHasTurnFinished(t *testing.T) {
	screenDone := "[ CQ muse -> gemini ]\nDone with turn.\n[ muse over ]\n"
	if !HasTurnFinished(screenDone, "muse") {
		t.Errorf("expected finished for muse over")
	}

	screenNotDone := "[ CQ muse -> gemini ]\nStill thinking..."
	if HasTurnFinished(screenNotDone, "muse") {
		t.Errorf("expected not finished")
	}
}
