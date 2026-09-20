package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cpro/agent-pair/pkg/mux"
)

func setupTestCacheDir(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tempDir)
	t.Setenv("HOME", tempDir)
	return tempDir
}

func TestSessionRoundTrip(t *testing.T) {
	setupTestCacheDir(t)

	if Exists() {
		t.Fatalf("expected session not to exist initially")
	}

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "no active pair session found") {
		t.Fatalf("expected 'no active pair session found' error, got %v", err)
	}

	now := time.Now().Truncate(time.Millisecond)
	sess := &Session{
		ID:      "pair-12345",
		MuxName: "tmux",
		PaneHandle: &mux.PaneHandle{
			MuxName:     "tmux",
			PaneID:      "%1",
			SessionName: "main",
		},
		LeaderAgent:      "agy",
		LeaderCallsign:   "gemini",
		FollowerAgent:    "opencode",
		FollowerCallsign: "muse",
		FollowerModel:    "opencode/muse-spark-1.3",
		Cwd:              "/path/to/project",
		ReadOnly:         true,
		CreatedAt:        now,
	}

	if err := Save(sess); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	if !Exists() {
		t.Fatalf("expected session to exist after Save()")
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.ID != sess.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, sess.ID)
	}
	if loaded.MuxName != sess.MuxName {
		t.Errorf("MuxName = %q, want %q", loaded.MuxName, sess.MuxName)
	}
	if loaded.PaneHandle == nil || loaded.PaneHandle.PaneID != sess.PaneHandle.PaneID {
		t.Errorf("PaneHandle mismatch: %+v", loaded.PaneHandle)
	}
	if loaded.LeaderAgent != sess.LeaderAgent || loaded.LeaderCallsign != sess.LeaderCallsign {
		t.Errorf("Leader mismatch")
	}
	if loaded.FollowerAgent != sess.FollowerAgent || loaded.FollowerCallsign != sess.FollowerCallsign {
		t.Errorf("Follower mismatch")
	}
	if loaded.FollowerModel != sess.FollowerModel {
		t.Errorf("FollowerModel = %q, want %q", loaded.FollowerModel, sess.FollowerModel)
	}
	if loaded.Cwd != sess.Cwd {
		t.Errorf("Cwd = %q, want %q", loaded.Cwd, sess.Cwd)
	}
	if loaded.ReadOnly != sess.ReadOnly {
		t.Errorf("ReadOnly = %v, want %v", loaded.ReadOnly, sess.ReadOnly)
	}
	if !loaded.CreatedAt.Equal(sess.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", loaded.CreatedAt, sess.CreatedAt)
	}

	// Test Clear
	if err := Clear(); err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}

	if Exists() {
		t.Fatalf("expected session not to exist after Clear()")
	}

	// Calling Clear again when file doesn't exist should succeed
	if err := Clear(); err != nil {
		t.Fatalf("subsequent Clear() failed: %v", err)
	}
}

func TestLoadCorruptSession(t *testing.T) {
	setupTestCacheDir(t)

	file, err := getSessionFile()
	if err != nil {
		t.Fatalf("getSessionFile() failed: %v", err)
	}

	if err := os.WriteFile(file, []byte("invalid json data!"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err = Load()
	if err == nil || !strings.Contains(err.Error(), "corrupt session file") {
		t.Fatalf("expected 'corrupt session file' error, got %v", err)
	}
}

func TestGetSessionFileMkdirError(t *testing.T) {
	tempDir := t.TempDir()
	// Create a regular file where the agent-pair directory would be
	blockerFile := filepath.Join(tempDir, "agent-pair")
	if err := os.WriteFile(blockerFile, []byte("blocking file"), 0644); err != nil {
		t.Fatalf("failed to create blocker file: %v", err)
	}

	t.Setenv("XDG_CACHE_HOME", tempDir)
	t.Setenv("HOME", tempDir)

	_, err := getSessionFile()
	if err == nil {
		t.Fatalf("expected error from getSessionFile when cache dir cannot be created")
	}

	// Test Save, Load, Clear, Exists with this error
	sess := &Session{ID: "test"}
	if err := Save(sess); err == nil {
		t.Errorf("expected Save to fail")
	}
	if _, err := Load(); err == nil {
		t.Errorf("expected Load to fail")
	}
	if err := Clear(); err == nil {
		t.Errorf("expected Clear to fail")
	}
	if Exists() {
		t.Errorf("expected Exists to be false")
	}
}

func TestGetSessionFileFallbackHome(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", tempHome)

	file, err := getSessionFile()
	if err != nil {
		t.Fatalf("getSessionFile() failed: %v", err)
	}

	expectedPrefix := filepath.Join(tempHome, ".cache", "agent-pair")
	if !strings.HasPrefix(file, expectedPrefix) {
		t.Errorf("file %q does not have expected prefix %q", file, expectedPrefix)
	}
}

func TestSessionIsScopedToLeaderPane(t *testing.T) {
	setupTestCacheDir(t)
	t.Setenv("TMUX_PANE", "%1")
	if err := Save(&Session{ID: "pane-one"}); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	t.Setenv("TMUX_PANE", "%2")
	if Exists() {
		t.Fatal("session from another leader pane leaked into this scope")
	}

	t.Setenv("TMUX_PANE", "%1")
	if !Exists() {
		t.Fatal("session was not found in its original leader pane")
	}
}
