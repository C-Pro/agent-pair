package agent

import (
	"fmt"
)

// LaunchOptions provides settings for constructing an agent CLI command.
type LaunchOptions struct {
	Model    string
	ReadOnly bool
	Cwd      string
	Callsign string
}

// AgentAdapter abstracts agent CLI invocations and status parsing.
type AgentAdapter interface {
	Name() string
	Available() bool
	DefaultCallsign(model string) string
	BuildLaunchCommand(opts LaunchOptions) ([]string, map[string]string, error)
	IsReady(screenOutput string) bool
	IsTurnFinished(screenOutput string, callsign string) bool
	StopCommand() string
	ListModels() ([]string, error)
}

var registry = make(map[string]AgentAdapter)

// Register registers an agent adapter.
func Register(a AgentAdapter) {
	registry[a.Name()] = a
}

// Get returns the agent adapter with the given name.
func Get(name string) (AgentAdapter, error) {
	if a, ok := registry[name]; ok {
		if !a.Available() {
			return nil, fmt.Errorf("agent binary for %q is not installed or available on PATH", name)
		}
		return a, nil
	}
	return nil, fmt.Errorf("unknown agent %q (supported: agy, opencode, claude, codex)", name)
}

// List returns names of all registered agents.
func List() []string {
	var names []string
	for k := range registry {
		names = append(names, k)
	}
	return names
}
