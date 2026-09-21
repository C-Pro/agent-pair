package agent

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// LaunchOptions provides settings for constructing an agent CLI command.
type LaunchOptions struct {
	Model    string
	Effort   string
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
	IsTurnFinished(screenOutput string, callsign string, turnID string) bool
	StopCommand() string
	ListModels() ([]string, error)
}

var registry = make(map[string]AgentAdapter)

// supportsFlag reports whether a CLI advertises a flag in its --help output.
// Hardening flags are added only when the installed binary understands them, so
// an older agent still launches instead of failing on an unknown flag.
func supportsFlag(bin, flag string) bool {
	helpCacheMu.Lock()
	help, ok := helpCache[bin]
	if !ok {
		cmd := exec.Command(bin, "--help")
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) == 0 {
			help = ""
		} else {
			help = string(out)
		}
		helpCache[bin] = help
	}
	helpCacheMu.Unlock()
	return strings.Contains(help, flag)
}

var (
	helpCacheMu sync.Mutex
	helpCache   = map[string]string{}
)

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

// SanitizeLaunchCommand wraps a command with environment variable unsets
// for outer terminal emulators (Kitty, Ghostty, WezTerm) so that background/split panes
// do not trigger escape-sequence capability queries that leak into the active pane.
func SanitizeLaunchCommand(cmd []string) []string {
	prefix := []string{
		"env",
		"-u", "KITTY_WINDOW_ID",
		"-u", "KITTY_PID",
		"-u", "KITTY_PUBLIC_KEY",
		"-u", "KITTY_SHELL_INTEGRATION",
		"-u", "KITTY_LISTEN_ON",
		"-u", "KITTY_INSTALLATION_DIR",
		"-u", "GHOSTTY_RESOURCES_DIR",
		"-u", "WEZTERM_PANE",
		"-u", "WEZTERM_EXECUTABLE",
	}
	return append(prefix, cmd...)
}

// DeriveCallsign extracts a clean, standardized callsign from a model identifier.
// When no model is provided, it falls back to the provided default/agent name.
func DeriveCallsign(model string, fallback string) string {
	if model == "" {
		return fallback
	}

	// 1. Remove provider prefix (e.g., "opencode/", "Lemonade/", "ollama/", "anthropic/")
	s := model
	if idx := strings.LastIndex(s, "/"); idx != -1 {
		s = s[idx+1:]
	}

	// 2. Lowercase
	s = strings.ToLower(s)

	// 3. Strip common noise suffixes
	noiseSuffixes := []string{
		"-contributor-free",
		"-free",
		"-latest",
		"-preview",
		"-it-mtp-gguf",
		"-mtp-gguf",
		"-gguf-q4_k_m",
		"-gguf-ud-q4_k_xl",
		"-gguf",
		"-it",
	}
	for _, suffix := range noiseSuffixes {
		if strings.HasSuffix(s, suffix) {
			s = strings.TrimSuffix(s, suffix)
			break
		}
	}

	// 4. Replace colons, underscores, or spaces with hyphens
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")

	// 5. Clean up duplicate hyphens
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-.")

	if s == "" {
		return fallback
	}
	return s
}
