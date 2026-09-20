package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cpro/agent-pair/pkg/mux"
)

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
	return filepath.Join(dir, "session.json"), nil
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
			return nil, fmt.Errorf("no active pair session found (run 'agent-pair start' first)")
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
	_ = os.Remove(file)
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
