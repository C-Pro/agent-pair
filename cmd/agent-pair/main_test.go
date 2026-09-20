package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cpro/agent-pair/pkg/mux"
	"github.com/cpro/agent-pair/pkg/session"
)

func setupTestEnv(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tempDir)
	t.Setenv("HOME", tempDir)

	// Create mock binaries for tmux and opencode
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	tmuxScript := `#!/bin/sh
cmd="$1"
shift
case "$cmd" in
  split-window)
    echo "%42"
    ;;
  capture-pane)
    echo "OpenCode Ask anything\n[ CQ muse -> gemini ]\nReady for pair programming as muse. Standing by.\n[ muse over ]"
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "tmux"), []byte(tmuxScript), 0755); err != nil {
		t.Fatalf("failed to write mock tmux: %v", err)
	}

	opencodeScript := `#!/bin/sh
if [ "$1" = "models" ]; then
  echo "opencode/mock-model-1"
  echo "opencode/mock-model-2"
  exit 0
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "opencode"), []byte(opencodeScript), 0755); err != nil {
		t.Fatalf("failed to write mock opencode: %v", err)
	}

	agyScript := `#!/bin/sh
if [ "$1" = "models" ]; then
  echo "gemini-flash"
  exit 0
fi
exit 0
`
	if err := os.WriteFile(filepath.Join(binDir, "agy"), []byte(agyScript), 0755); err != nil {
		t.Fatalf("failed to write mock agy: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX", "/tmp/tmux-mock/default,1,0")
	return tempDir
}

func TestReadMessageFromArgsOrStdin(t *testing.T) {
	t.Run("from args", func(t *testing.T) {
		msg, err := readMessageFromArgsOrStdin([]string{"hello", "world", "pair"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msg != "hello world pair" {
			t.Errorf("got %q, want 'hello world pair'", msg)
		}
	})

	t.Run("from pipe stdin", func(t *testing.T) {
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe failed: %v", err)
		}
		w.WriteString("piped message test")
		w.Close()
		os.Stdin = r

		msg, err := readMessageFromArgsOrStdin(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msg != "piped message test" {
			t.Errorf("got %q, want 'piped message test'", msg)
		}
	})

	t.Run("empty pipe stdin", func(t *testing.T) {
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe failed: %v", err)
		}
		w.Close()
		os.Stdin = r

		msg, err := readMessageFromArgsOrStdin(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if msg != "" {
			t.Errorf("got %q, want ''", msg)
		}
	})
}

func TestPrintUsage(t *testing.T) {
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	printUsage()
	w.Close()

	var buf bytes.Buffer
	io.Copy(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "agent-pair: Multi-Agent Pair Programming CLI") {
		t.Errorf("usage output missing title")
	}
	if !strings.Contains(out, "COMMANDS:") || !strings.Contains(out, "start") {
		t.Errorf("usage output missing commands")
	}
}

func TestRunModels(t *testing.T) {
	setupTestEnv(t)

	// Default agent (opencode)
	if err := runModels(nil); err != nil {
		t.Fatalf("runModels(nil) failed: %v", err)
	}

	// Explicit claude
	if err := runModels([]string{"claude"}); err != nil {
		t.Fatalf("runModels(claude) failed: %v", err)
	}

	// Unknown agent
	if err := runModels([]string{"unknown-agent"}); err == nil {
		t.Errorf("expected error for unknown agent")
	}
}

func TestRunStatus(t *testing.T) {
	setupTestEnv(t)

	// When no session exists
	if err := runStatus(nil); err == nil || !strings.Contains(err.Error(), "no active pair session") {
		t.Errorf("expected no active pair session error, got %v", err)
	}

	// Save active session
	sess := &session.Session{
		ID:      "pair-test-1",
		MuxName: "tmux",
		PaneHandle: &mux.PaneHandle{
			MuxName: "tmux",
			PaneID:  "%42",
		},
		LeaderAgent:      "agy",
		LeaderCallsign:   "gemini",
		FollowerAgent:    "opencode",
		FollowerCallsign: "muse",
		FollowerModel:    "opencode/muse-spark-1.3",
		Cwd:              "/tmp/test",
		ReadOnly:         true,
		CreatedAt:        time.Now().Add(-10 * time.Second),
	}
	if err := session.Save(sess); err != nil {
		t.Fatalf("session.Save failed: %v", err)
	}

	if err := runStatus(nil); err != nil {
		t.Fatalf("runStatus failed: %v", err)
	}

	// With unknown mux in session
	sess.MuxName = "unknown-mux"
	_ = session.Save(sess)
	if err := runStatus(nil); err == nil || !strings.Contains(err.Error(), "unknown multiplexer") {
		t.Errorf("expected unknown multiplexer error, got %v", err)
	}
}

func TestRunSend(t *testing.T) {
	setupTestEnv(t)

	// No message provided
	if err := runSend(nil); err == nil {
		t.Errorf("expected error when no message provided")
	}

	// No session exists
	if err := runSend([]string{"hello"}); err == nil || !strings.Contains(err.Error(), "no active pair session") {
		t.Errorf("expected no active pair session error, got %v", err)
	}

	// Valid session
	sess := &session.Session{
		ID:      "pair-test-1",
		MuxName: "tmux",
		PaneHandle: &mux.PaneHandle{
			MuxName: "tmux",
			PaneID:  "%42",
		},
		LeaderAgent:      "agy",
		LeaderCallsign:   "gemini",
		FollowerAgent:    "opencode",
		FollowerCallsign: "muse",
		FollowerModel:    "opencode/muse-spark-1.3",
		Cwd:              "/tmp/test",
		CreatedAt:        time.Now(),
	}
	if err := session.Save(sess); err != nil {
		t.Fatalf("session.Save failed: %v", err)
	}

	if err := runSend([]string{"hello", "world"}); err != nil {
		t.Fatalf("runSend failed: %v", err)
	}
}

func TestRunWait(t *testing.T) {
	setupTestEnv(t)

	// No session
	if err := runWait([]string{"-timeout", "1"}); err == nil || !strings.Contains(err.Error(), "no active pair session") {
		t.Errorf("expected no active session error, got %v", err)
	}

	// Valid session with immediate response from mock tmux
	sess := &session.Session{
		ID:      "pair-test-1",
		MuxName: "tmux",
		PaneHandle: &mux.PaneHandle{
			MuxName: "tmux",
			PaneID:  "%42",
		},
		LeaderAgent:      "agy",
		LeaderCallsign:   "gemini",
		FollowerAgent:    "opencode",
		FollowerCallsign: "muse",
		FollowerModel:    "opencode/muse-spark-1.3",
		Cwd:              "/tmp/test",
		CreatedAt:        time.Now(),
	}
	if err := session.Save(sess); err != nil {
		t.Fatalf("session.Save failed: %v", err)
	}

	// Wait with extraction
	if err := runWait([]string{"-timeout", "2"}); err != nil {
		t.Fatalf("runWait failed: %v", err)
	}

	// Wait with raw flag
	if err := runWait([]string{"-timeout", "2", "-raw"}); err != nil {
		t.Fatalf("runWait -raw failed: %v", err)
	}
}

func TestRunTurn(t *testing.T) {
	setupTestEnv(t)

	// No message error
	if err := runTurn(nil); err == nil {
		t.Errorf("expected error when no message in runTurn")
	}

	// Valid session
	sess := &session.Session{
		ID:      "pair-test-1",
		MuxName: "tmux",
		PaneHandle: &mux.PaneHandle{
			MuxName: "tmux",
			PaneID:  "%42",
		},
		LeaderAgent:      "agy",
		LeaderCallsign:   "gemini",
		FollowerAgent:    "opencode",
		FollowerCallsign: "muse",
		FollowerModel:    "opencode/muse-spark-1.3",
		Cwd:              "/tmp/test",
		CreatedAt:        time.Now(),
	}
	if err := session.Save(sess); err != nil {
		t.Fatalf("session.Save failed: %v", err)
	}

	if err := runTurn([]string{"What", "is", "the", "plan?"}); err != nil {
		t.Fatalf("runTurn failed: %v", err)
	}
}

func TestRunStop(t *testing.T) {
	setupTestEnv(t)

	// No session
	if err := runStop(nil); err == nil || !strings.Contains(err.Error(), "no active pair session") {
		t.Errorf("expected no active session error, got %v", err)
	}

	// Valid session
	sess := &session.Session{
		ID:      "pair-test-1",
		MuxName: "tmux",
		PaneHandle: &mux.PaneHandle{
			MuxName: "tmux",
			PaneID:  "%42",
		},
		LeaderAgent:      "agy",
		LeaderCallsign:   "gemini",
		FollowerAgent:    "opencode",
		FollowerCallsign: "muse",
		FollowerModel:    "opencode/muse-spark-1.3",
		Cwd:              "/tmp/test",
		CreatedAt:        time.Now(),
	}
	if err := session.Save(sess); err != nil {
		t.Fatalf("session.Save failed: %v", err)
	}

	if err := runStop(nil); err != nil {
		t.Fatalf("runStop failed: %v", err)
	}

	if session.Exists() {
		t.Errorf("expected session to be cleared after runStop")
	}

	// Second stop should fail because session is cleared
	if err := runStop(nil); err == nil {
		t.Errorf("expected second runStop to fail")
	}
}

func TestRunStart(t *testing.T) {
	setupTestEnv(t)

	// Test validation errors
	t.Run("unknown multiplexer", func(t *testing.T) {
		err := runStart([]string{"-mux", "nonexistent"})
		if err == nil || !strings.Contains(err.Error(), "unknown multiplexer") {
			t.Errorf("expected unknown multiplexer error, got %v", err)
		}
	})

	t.Run("unknown follower", func(t *testing.T) {
		err := runStart([]string{"-follower", "nonexistent"})
		if err == nil || !strings.Contains(err.Error(), "unknown agent") {
			t.Errorf("expected unknown agent error, got %v", err)
		}
	})

	t.Run("unknown leader", func(t *testing.T) {
		err := runStart([]string{"-leader", "nonexistent"})
		if err == nil || !strings.Contains(err.Error(), "unknown agent") {
			t.Errorf("expected unknown agent error, got %v", err)
		}
	})

	t.Run("successful start with mock", func(t *testing.T) {
		err := runStart([]string{
			"-mux", "tmux",
			"-follower", "opencode",
			"-leader", "agy",
			"-timeout", "5",
			"-leader-callsign", "test-leader",
			"-follower-callsign", "test-follower",
		})
		if err != nil {
			t.Fatalf("runStart failed: %v", err)
		}

		if !session.Exists() {
			t.Fatalf("expected session to exist after start")
		}

		// Existing session without --force should fail
		err2 := runStart([]string{"-mux", "tmux"})
		if err2 == nil || !strings.Contains(err2.Error(), "an active session already exists") {
			t.Errorf("expected 'an active session already exists' error, got %v", err2)
		}

		// Existing session WITH --force should succeed
		err3 := runStart([]string{"-mux", "tmux", "-force", "-timeout", "5", "-no-bootstrap"})
		if err3 != nil {
			t.Errorf("expected --force to succeed, got %v", err3)
		}
	})
}

func TestMainProcessHelper(t *testing.T) {
	if os.Getenv("GO_TEST_SUBPROCESS") != "1" {
		return
	}
	// Run main() with arguments passed after --
	args := []string{"agent-pair"}
	for i, arg := range os.Args {
		if arg == "--" {
			args = append(args, os.Args[i+1:]...)
			break
		}
	}
	os.Args = args
	main()
}

func runMainSubprocess(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmdArgs := []string{"-test.run=TestMainProcessHelper", "--"}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "GO_TEST_SUBPROCESS=1", "XDG_CACHE_HOME="+t.TempDir())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run subprocess: %v", err)
		}
	}

	return stdout.String(), stderr.String(), exitCode
}

func TestMainDispatch(t *testing.T) {
	t.Run("no args prints usage and exits 1", func(t *testing.T) {
		stdout, _, code := runMainSubprocess(t)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stdout, "agent-pair: Multi-Agent Pair Programming CLI") {
			t.Errorf("expected usage on stdout, got: %s", stdout)
		}
	})

	t.Run("help prints usage and exits 0", func(t *testing.T) {
		stdout, _, code := runMainSubprocess(t, "help")
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout, "agent-pair: Multi-Agent Pair Programming CLI") {
			t.Errorf("expected usage on stdout, got: %s", stdout)
		}
	})

	t.Run("--help prints usage and exits 0", func(t *testing.T) {
		stdout, _, code := runMainSubprocess(t, "--help")
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout, "agent-pair: Multi-Agent Pair Programming CLI") {
			t.Errorf("expected usage on stdout, got: %s", stdout)
		}
	})

	t.Run("unknown command exits 1 and prints error", func(t *testing.T) {
		_, stderr, code := runMainSubprocess(t, "unknown-cmd")
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr, "Unknown command: unknown-cmd") {
			t.Errorf("expected unknown command in stderr, got: %s", stderr)
		}
	})

	t.Run("command error exits 1 with Error prefix", func(t *testing.T) {
		_, stderr, code := runMainSubprocess(t, "status")
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr, "Error:") {
			t.Errorf("expected Error: prefix on stderr, got: %s", stderr)
		}
	})
}
