package agent

import (
	"os/exec"
	"strings"

	"github.com/cpro/agent-pair/pkg/protocol"
)

type CodexAgent struct{}

func init() {
	Register(&CodexAgent{})
}

func (c *CodexAgent) Name() string {
	return "codex"
}

func (c *CodexAgent) Available() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

func (c *CodexAgent) DefaultCallsign(model string) string {
	return "codex"
}

func (c *CodexAgent) BuildLaunchCommand(opts LaunchOptions) ([]string, map[string]string, error) {
	// Use inline mode to keep clean scrollback
	cmd := []string{"codex", "--no-alt-screen"}

	if opts.Model != "" {
		cmd = append(cmd, "-m", opts.Model)
	}

	if opts.ReadOnly {
		cmd = append(cmd, "-s", "read-only")
	}

	return cmd, nil, nil
}

func (c *CodexAgent) IsReady(screenOutput string) bool {
	clean := protocol.CleanTUIArtifacts(screenOutput)
	return strings.Contains(clean, "Codex") ||
		strings.Contains(clean, "OpenAI") ||
		strings.Contains(clean, "> ")
}

func (c *CodexAgent) IsTurnFinished(screenOutput string, callsign string) bool {
	return protocol.HasTurnFinished(screenOutput, callsign)
}

func (c *CodexAgent) StopCommand() string {
	return "exit"
}

func (c *CodexAgent) ListModels() ([]string, error) {
	// Standard models supported by Codex
	return []string{
		"o3",
		"o3-mini",
		"gpt-4o",
	}, nil
}
