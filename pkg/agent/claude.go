package agent

import (
	"os/exec"
	"strings"

	"github.com/cpro/agent-pair/pkg/protocol"
)

type ClaudeAgent struct{}

func init() {
	Register(&ClaudeAgent{})
}

func (c *ClaudeAgent) Name() string {
	return "claude"
}

func (c *ClaudeAgent) Available() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func (c *ClaudeAgent) DefaultCallsign(model string) string {
	return "claude"
}

func (c *ClaudeAgent) BuildLaunchCommand(opts LaunchOptions) ([]string, map[string]string, error) {
	cmd := []string{"claude"}

	if opts.Model != "" {
		cmd = append(cmd, "--model", opts.Model)
	}

	if opts.ReadOnly {
		// Native plan mode in Claude Code with edit/write/bash explicitly disallowed
		cmd = append(cmd, "--permission-mode", "plan", "--disallowed-tools", "Edit,Write,Bash")
	}

	return cmd, nil, nil
}

func (c *ClaudeAgent) IsReady(screenOutput string) bool {
	clean := protocol.CleanTUIArtifacts(screenOutput)
	return strings.Contains(clean, "Claude Code") ||
		strings.Contains(clean, "Welcome to Claude Code")
}

func (c *ClaudeAgent) IsTurnFinished(screenOutput string, callsign string) bool {
	return protocol.HasTurnFinished(screenOutput, callsign)
}

func (c *ClaudeAgent) StopCommand() string {
	return "/exit"
}

func (c *ClaudeAgent) ListModels() ([]string, error) {
	// Standard models supported by Claude Code
	return []string{
		"claude-3-7-sonnet-latest",
		"claude-3-5-sonnet-latest",
		"claude-3-5-haiku-latest",
	}, nil
}
