package session

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cpro/agent-pair/pkg/mux"
)

var ErrNoActiveSession = errors.New("no active pair session found (run 'agent-pair start' first)")

// Session stores active pair programming session metadata.
type Session struct {
	ID               string          `json:"id"`
	MuxName          string          `json:"mux_name"`
	PaneHandle       *mux.PaneHandle `json:"pane_handle"`
	LeaderAgent      string          `json:"leader_agent"`
	LeaderCallsign   string          `json:"leader_callsign"`
	FollowerAgent    string          `json:"follower_agent"`
	FollowerCallsign string          `json:"follower_callsign"`
	FollowerModel    string          `json:"follower_model"`
	Cwd              string          `json:"cwd"`
	ReadOnly         bool            `json:"read_only"`
	CreatedAt        time.Time       `json:"created_at"`
	ResponseBaseline int             `json:"response_marker_baseline,omitempty"`
}

func getSessionFile() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = filepath.Join(os.Getenv("HOME"), ".cache")
	}
	dir := filepath.Join(cacheDir, "agent-pair")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	scope := sessionScope()
	digest := sha256.Sum256([]byte(scope))
	return filepath.Join(dir, fmt.Sprintf("session-%x.json", digest[:8])), nil
}

func sessionScope() string {
	for _, key := range []string{"TMUX_PANE", "ZELLIJ_PANE_ID", "HERDR_PANE_ID"} {
		if value := os.Getenv(key); value != "" {
			return key + ":" + value
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return "cwd:" + filepath.Clean(cwd)
	}
	return "user-default"
}

// Save writes the session to disk.
func Save(s *Session) error {
	file, err := getSessionFile()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(file, data, 0644)
}

// Load reads the session from disk.
func Load() (*Session, error) {
	file, err := getSessionFile()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoActiveSession
		}
		return nil, err
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("corrupt session file: %w", err)
	}

	return &s, nil
}

// Clear removes the session file.
func Clear() error {
	file, err := getSessionFile()
	if err != nil {
		return err
	}
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Exists checks if an active session file exists.
func Exists() bool {
	file, err := getSessionFile()
	if err != nil {
		return false
	}
	_, err = os.Stat(file)
	return err == nil
}
