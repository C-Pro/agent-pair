package mux

import (
	"fmt"
)

// PaneHandle represents a created pane across any multiplexer.
type PaneHandle struct {
	MuxName     string `json:"mux_name"`
	PaneID      string `json:"pane_id"`
	SessionName string `json:"session_name,omitempty"`
}

// PaneOptions configures how a pane is created.
type PaneOptions struct {
	Title     string
	Cwd       string
	Command   []string
	Direction string // "horizontal" or "vertical"
	Size      int    // percentage, e.g. 45
}

// Multiplexer abstracts terminal multiplexer / screen demuxer operations.
type Multiplexer interface {
	Name() string
	Available() bool
	DetectActive() bool
	CreatePane(opts PaneOptions) (*PaneHandle, error)
	SendText(handle *PaneHandle, text string) error
	CaptureOutput(handle *PaneHandle) (string, error)
	ClosePane(handle *PaneHandle) error
}

var registry = make(map[string]Multiplexer)

// Register registers a multiplexer implementation.
func Register(m Multiplexer) {
	registry[m.Name()] = m
}

// Get returns the multiplexer with the given name.
func Get(name string) (Multiplexer, error) {
	if m, ok := registry[name]; ok {
		if !m.Available() {
			return nil, fmt.Errorf("multiplexer %q is not installed or available on PATH", name)
		}
		return m, nil
	}
	return nil, fmt.Errorf("unknown multiplexer %q (supported: tmux, zellij, herdr)", name)
}

// DetectActive detects which multiplexer is currently running in the active terminal environment.
func DetectActive() (Multiplexer, error) {
	// First priority: check active environments
	for _, name := range []string{"tmux", "zellij", "herdr"} {
		if m, ok := registry[name]; ok && m.Available() && m.DetectActive() {
			return m, nil
		}
	}

	// Second priority: check if any multiplexer server is running
	for _, name := range []string{"tmux", "zellij", "herdr"} {
		if m, ok := registry[name]; ok && m.Available() {
			return m, nil
		}
	}

	return nil, fmt.Errorf("no supported terminal multiplexer (tmux, zellij, herdr) detected or active")
}

// DetectOrGet resolves multiplexer by name or auto-detects if "auto" / empty.
func DetectOrGet(name string) (Multiplexer, error) {
	if name == "" || name == "auto" {
		return DetectActive()
	}
	return Get(name)
}
