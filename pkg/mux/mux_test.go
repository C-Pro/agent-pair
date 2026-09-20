package mux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockMultiplexer struct {
	name      string
	available bool
	active    bool
}

func (m *mockMultiplexer) Name() string {
	return m.name
}

func (m *mockMultiplexer) Available() bool {
	return m.available
}

func (m *mockMultiplexer) DetectActive() bool {
	return m.active
}

func (m *mockMultiplexer) CreatePane(opts PaneOptions) (*PaneHandle, error) {
	return &PaneHandle{MuxName: m.name, PaneID: "mock-pane"}, nil
}

func (m *mockMultiplexer) PaneAlive(handle *PaneHandle) (bool, error) {
	return handle != nil && handle.PaneID != "", nil
}

func (m *mockMultiplexer) SendText(handle *PaneHandle, text string) error {
	return nil
}

func (m *mockMultiplexer) CaptureOutput(handle *PaneHandle) (string, error) {
	return "mock output", nil
}

func (m *mockMultiplexer) ClosePane(handle *PaneHandle) error {
	return nil
}

func snapshotRegistry(t *testing.T) {
	t.Helper()
	orig := make(map[string]Multiplexer)
	for k, v := range registry {
		orig[k] = v
	}
	t.Cleanup(func() {
		registry = orig
	})
}

func TestMuxRegistry(t *testing.T) {
	snapshotRegistry(t)

	// Unknown multiplexer
	_, err := Get("nonexistent-mux")
	if err == nil || !strings.Contains(err.Error(), "unknown multiplexer") {
		t.Errorf("expected unknown multiplexer error, got %v", err)
	}

	// Register a mock unavailable multiplexer
	mockUnavail := &mockMultiplexer{name: "test-unavail", available: false}
	Register(mockUnavail)

	_, err = Get("test-unavail")
	if err == nil || !strings.Contains(err.Error(), "not installed or available") {
		t.Errorf("expected not installed or available error, got %v", err)
	}

	// Register a mock available multiplexer
	mockAvail := &mockMultiplexer{name: "test-avail", available: true}
	Register(mockAvail)

	m, err := Get("test-avail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name() != "test-avail" {
		t.Errorf("got %q, want 'test-avail'", m.Name())
	}
}

func TestDetectActive(t *testing.T) {
	snapshotRegistry(t)

	// Case 1: No multiplexer available
	registry = map[string]Multiplexer{
		"tmux":   &mockMultiplexer{name: "tmux", available: false},
		"zellij": &mockMultiplexer{name: "zellij", available: false},
		"herdr":  &mockMultiplexer{name: "herdr", available: false},
	}
	_, err := DetectActive()
	if err == nil || !strings.Contains(err.Error(), "not running inside") {
		t.Errorf("expected no supported terminal multiplexer error, got %v", err)
	}

	// Case 2: One active multiplexer detected
	registry = map[string]Multiplexer{
		"tmux":   &mockMultiplexer{name: "tmux", available: true, active: false},
		"zellij": &mockMultiplexer{name: "zellij", available: true, active: true},
		"herdr":  &mockMultiplexer{name: "herdr", available: true, active: false},
	}
	m, err := DetectActive()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name() != "zellij" {
		t.Errorf("expected zellij to be detected active, got %s", m.Name())
	}

	// Case 3: Priority when multiple active: tmux wins
	registry = map[string]Multiplexer{
		"tmux":   &mockMultiplexer{name: "tmux", available: true, active: true},
		"zellij": &mockMultiplexer{name: "zellij", available: true, active: true},
		"herdr":  &mockMultiplexer{name: "herdr", available: true, active: true},
	}
	m, err = DetectActive()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name() != "tmux" {
		t.Errorf("expected tmux to win active priority, got %s", m.Name())
	}

	// Case 4: Installed but inactive multiplexers must not be selected.
	registry = map[string]Multiplexer{
		"tmux":   &mockMultiplexer{name: "tmux", available: false, active: false},
		"zellij": &mockMultiplexer{name: "zellij", available: true, active: false},
		"herdr":  &mockMultiplexer{name: "herdr", available: true, active: false},
	}
	if _, err = DetectActive(); err == nil || !strings.Contains(err.Error(), "not running inside") {
		t.Errorf("expected inactive multiplexer error, got %v", err)
	}
}

func TestDetectOrGet(t *testing.T) {
	snapshotRegistry(t)

	registry = map[string]Multiplexer{
		"tmux": &mockMultiplexer{name: "tmux", available: true, active: true},
	}

	// "" should call DetectActive
	m1, err := DetectOrGet("")
	if err != nil || m1.Name() != "tmux" {
		t.Errorf("DetectOrGet('') failed: %v", err)
	}

	// "auto" should call DetectActive
	m2, err := DetectOrGet("auto")
	if err != nil || m2.Name() != "tmux" {
		t.Errorf("DetectOrGet('auto') failed: %v", err)
	}

	// Specific name should call Get
	m3, err := DetectOrGet("tmux")
	if err != nil || m3.Name() != "tmux" {
		t.Errorf("DetectOrGet('tmux') failed: %v", err)
	}
}

func TestTmuxMuxWhenUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	tm := &TmuxMux{}

	if tm.Name() != "tmux" {
		t.Errorf("Name() = %q, want 'tmux'", tm.Name())
	}
	if tm.Available() {
		t.Errorf("expected Available() = false in empty PATH")
	}

	handle := &PaneHandle{MuxName: "tmux", PaneID: "%99"}
	opts := PaneOptions{Title: "test"}

	if _, err := tm.CreatePane(opts); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for CreatePane, got %v", err)
	}
	if err := tm.SendText(handle, "hello"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for SendText, got %v", err)
	}
	if _, err := tm.CaptureOutput(handle); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for CaptureOutput, got %v", err)
	}
	if err := tm.ClosePane(handle); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for ClosePane, got %v", err)
	}
}

func TestTmuxDetectActive(t *testing.T) {
	tm := &TmuxMux{}
	t.Setenv("TMUX", "")
	if tm.DetectActive() {
		t.Errorf("expected DetectActive = false when TMUX is empty")
	}
	t.Setenv("TMUX", "/tmp/tmux-test/session")
	if !tm.DetectActive() {
		t.Errorf("expected DetectActive = true when TMUX is set")
	}
}

func TestShellCommandQuotesEveryArgument(t *testing.T) {
	got := shellCommand([]string{"agy", "--model", "gemini; touch /tmp/pwned", "it's-safe"})
	want := `exec 'agy' '--model' 'gemini; touch /tmp/pwned' 'it'"'"'s-safe'`
	if got != want {
		t.Fatalf("shellCommand() = %q, want %q", got, want)
	}
}

func TestZellijMuxWhenUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	zm := &ZellijMux{}

	if zm.Name() != "zellij" {
		t.Errorf("Name() = %q, want 'zellij'", zm.Name())
	}
	if zm.Available() {
		t.Errorf("expected Available() = false in empty PATH")
	}

	handle := &PaneHandle{MuxName: "zellij", PaneID: "pane-1"}
	opts := PaneOptions{Title: "test"}

	if _, err := zm.CreatePane(opts); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for CreatePane, got %v", err)
	}
	if err := zm.SendText(handle, "hello"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for SendText, got %v", err)
	}
	if _, err := zm.CaptureOutput(handle); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for CaptureOutput, got %v", err)
	}
	if err := zm.ClosePane(handle); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for ClosePane, got %v", err)
	}
}

func TestZellijDetectActive(t *testing.T) {
	zm := &ZellijMux{}
	t.Setenv("ZELLIJ", "")
	if zm.DetectActive() {
		t.Errorf("expected DetectActive = false when ZELLIJ is empty")
	}
	t.Setenv("ZELLIJ", "0")
	if !zm.DetectActive() {
		t.Errorf("expected DetectActive = true when ZELLIJ is set")
	}
}

func TestZellijPaneRegex(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"terminal_42", "terminal_42"},
		{"99", "99"},
		{"created pane terminal_12 successfully", "terminal_12"},
		{"no pane id", ""},
	}

	for _, tt := range tests {
		match := zellijPaneRegex.FindString(tt.input)
		if match != tt.expected {
			t.Errorf("zellijPaneRegex.FindString(%q) = %q, want %q", tt.input, match, tt.expected)
		}
	}
}

func TestHerdrMuxWhenUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	hm := &HerdrMux{}

	if hm.Name() != "herdr" {
		t.Errorf("Name() = %q, want 'herdr'", hm.Name())
	}
	if hm.Available() {
		t.Errorf("expected Available() = false in empty PATH")
	}

	handle := &PaneHandle{MuxName: "herdr", PaneID: "h-1"}
	opts := PaneOptions{Title: "test"}

	if _, err := hm.CreatePane(opts); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for CreatePane, got %v", err)
	}
	if err := hm.SendText(handle, "hello"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for SendText, got %v", err)
	}
	if _, err := hm.CaptureOutput(handle); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for CaptureOutput, got %v", err)
	}
	if err := hm.ClosePane(handle); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected not installed error for ClosePane, got %v", err)
	}
}

func TestHerdrDetectActive(t *testing.T) {
	hm := &HerdrMux{}

	t.Run("with HERDR_PANE_ID", func(t *testing.T) {
		t.Setenv("HERDR_PANE_ID", "pane-123")
		t.Setenv("HERDR_SESSION", "")
		if !hm.DetectActive() {
			t.Errorf("expected DetectActive = true with HERDR_PANE_ID")
		}
	})

	t.Run("with HERDR_SESSION", func(t *testing.T) {
		t.Setenv("HERDR_PANE_ID", "")
		t.Setenv("HERDR_SESSION", "sess-1")
		if !hm.DetectActive() {
			t.Errorf("expected DetectActive = true with HERDR_SESSION")
		}
	})

	t.Run("without env and no server", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		t.Setenv("HERDR_PANE_ID", "")
		t.Setenv("HERDR_SESSION", "")
		if hm.DetectActive() {
			t.Errorf("expected DetectActive = false when not running")
		}
	})
}

func setupMockBinDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Mock tmux script
	tmuxScript := `#!/bin/sh
cmd="$1"
shift
case "$cmd" in
  split-window)
    echo "%42"
    ;;
  capture-pane)
    echo "tmux mock screen output"
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "tmux"), []byte(tmuxScript), 0755); err != nil {
		t.Fatalf("failed to write mock tmux: %v", err)
	}

	// Mock zellij script
	zellijScript := `#!/bin/sh
cmd="$1"
shift
case "$cmd" in
  run)
    echo "terminal_7"
    ;;
  action)
    sub="$1"
    if [ "$sub" = "dump-screen" ]; then
      echo "zellij mock screen output"
    fi
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "zellij"), []byte(zellijScript), 0755); err != nil {
		t.Fatalf("failed to write mock zellij: %v", err)
	}

	// Mock herdr script
	herdrScript := `#!/bin/sh
cmd="$1"
shift
case "$cmd" in
  pane)
    sub="$1"
    shift
    case "$sub" in
      split)
        echo "herdr-pane-99"
        ;;
      read)
        echo "herdr mock screen output"
        ;;
      *)
        exit 0
        ;;
    esac
    ;;
  status)
    echo "server running on port 1234"
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "herdr"), []byte(herdrScript), 0755); err != nil {
		t.Fatalf("failed to write mock herdr: %v", err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestTmuxMuxOperations(t *testing.T) {
	setupMockBinDir(t)
	tm := &TmuxMux{}

	if !tm.Available() {
		t.Fatalf("expected tm.Available() = true with mock binary")
	}

	// CreatePane horizontal
	optsH := PaneOptions{
		Title:     "test-title",
		Cwd:       t.TempDir(),
		Command:   []string{"echo", "hello"},
		Direction: "horizontal",
		Size:      40,
	}
	handleH, err := tm.CreatePane(optsH)
	if err != nil {
		t.Fatalf("CreatePane failed: %v", err)
	}
	if handleH.PaneID != "%42" {
		t.Errorf("got PaneID %q, want '%%42'", handleH.PaneID)
	}

	// CreatePane vertical / default size
	optsV := PaneOptions{
		Direction: "vertical",
	}
	handleV, err := tm.CreatePane(optsV)
	if err != nil {
		t.Fatalf("CreatePane vertical failed: %v", err)
	}
	if handleV.PaneID != "%42" {
		t.Errorf("got PaneID %q, want '%%42'", handleV.PaneID)
	}

	// SendText
	if err := tm.SendText(handleH, "test input"); err != nil {
		t.Fatalf("SendText failed: %v", err)
	}

	// CaptureOutput
	out, err := tm.CaptureOutput(handleH)
	if err != nil {
		t.Fatalf("CaptureOutput failed: %v", err)
	}
	if !strings.Contains(out, "tmux mock screen output") {
		t.Errorf("unexpected CaptureOutput: %q", out)
	}

	// ClosePane
	if err := tm.ClosePane(handleH); err != nil {
		t.Fatalf("ClosePane failed: %v", err)
	}
}

func TestZellijMuxOperations(t *testing.T) {
	setupMockBinDir(t)
	zm := &ZellijMux{}

	if !zm.Available() {
		t.Fatalf("expected zm.Available() = true with mock binary")
	}

	optsH := PaneOptions{
		Title:     "z-title",
		Cwd:       t.TempDir(),
		Command:   []string{"echo", "hello"},
		Direction: "horizontal",
	}
	handleH, err := zm.CreatePane(optsH)
	if err != nil {
		t.Fatalf("CreatePane failed: %v", err)
	}
	if handleH.PaneID != "terminal_7" {
		t.Errorf("got PaneID %q, want 'terminal_7'", handleH.PaneID)
	}

	optsV := PaneOptions{
		Direction: "down",
	}
	handleV, err := zm.CreatePane(optsV)
	if err != nil {
		t.Fatalf("CreatePane down failed: %v", err)
	}
	if handleV.PaneID != "terminal_7" {
		t.Errorf("got PaneID %q, want 'terminal_7'", handleV.PaneID)
	}

	// SendText
	if err := zm.SendText(handleH, "test message"); err != nil {
		t.Fatalf("SendText failed: %v", err)
	}

	// CaptureOutput
	out, err := zm.CaptureOutput(handleH)
	if err != nil {
		t.Fatalf("CaptureOutput failed: %v", err)
	}
	if !strings.Contains(out, "zellij mock screen output") {
		t.Errorf("unexpected CaptureOutput: %q", out)
	}

	// ClosePane
	if err := zm.ClosePane(handleH); err != nil {
		t.Fatalf("ClosePane failed: %v", err)
	}
	if err := zm.ClosePane(&PaneHandle{}); err != nil {
		t.Fatalf("ClosePane should be idempotent for an empty pane: %v", err)
	}
}

func TestHerdrMuxOperations(t *testing.T) {
	setupMockBinDir(t)
	hm := &HerdrMux{}

	if !hm.Available() {
		t.Fatalf("expected hm.Available() = true with mock binary")
	}

	if !hm.DetectActive() {
		t.Errorf("expected DetectActive = true with status server returning 'running'")
	}

	optsH := PaneOptions{
		Cwd:       t.TempDir(),
		Command:   []string{"echo", "hello"},
		Direction: "horizontal",
	}
	handleH, err := hm.CreatePane(optsH)
	if err != nil {
		t.Fatalf("CreatePane failed: %v", err)
	}
	if handleH.PaneID != "herdr-pane-99" {
		t.Errorf("got PaneID %q, want 'herdr-pane-99'", handleH.PaneID)
	}

	optsV := PaneOptions{
		Direction: "vertical",
	}
	handleV, err := hm.CreatePane(optsV)
	if err != nil {
		t.Fatalf("CreatePane vertical failed: %v", err)
	}
	if handleV.PaneID != "herdr-pane-99" {
		t.Errorf("got PaneID %q, want 'herdr-pane-99'", handleV.PaneID)
	}

	// SendText
	if err := hm.SendText(handleH, "herdr message"); err != nil {
		t.Fatalf("SendText failed: %v", err)
	}

	// CaptureOutput
	out, err := hm.CaptureOutput(handleH)
	if err != nil {
		t.Fatalf("CaptureOutput failed: %v", err)
	}
	if !strings.Contains(out, "herdr mock screen output") {
		t.Errorf("unexpected CaptureOutput: %q", out)
	}

	// ClosePane
	if err := hm.ClosePane(handleH); err != nil {
		t.Fatalf("ClosePane failed: %v", err)
	}
}
