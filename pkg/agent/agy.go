package agent

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cpro/agent-pair/pkg/protocol"
)

type AgyAgent struct{}

func init() {
	Register(&AgyAgent{})
}

func (a *AgyAgent) Name() string {
	return "agy"
}

func (a *AgyAgent) Available() bool {
	_, err := exec.LookPath("agy")
	return err == nil
}

func (a *AgyAgent) DefaultCallsign(model string) string {
	m := strings.ToLower(model)
	if strings.Contains(m, "claude") {
		return "claude"
	}
	if strings.Contains(m, "gpt") {
		return "gpt"
	}
	return "gemini"
}

func (a *AgyAgent) BuildLaunchCommand(opts LaunchOptions) ([]string, map[string]string, error) {
	cmd := []string{"agy"}

	if opts.Model != "" {
		cmd = append(cmd, "--model", opts.Model)
	}
	if opts.Effort != "" {
		switch opts.Effort {
		case "low", "medium", "high":
			cmd = append(cmd, "--effort", opts.Effort)
		default:
			return nil, nil, fmt.Errorf("invalid agy effort %q (available: low, medium, high)", opts.Effort)
		}
	}

	if opts.ReadOnly {
		cmd = append(cmd, "--mode", "plan", "--sandbox")
	}

	return cmd, nil, nil
}

func (a *AgyAgent) IsReady(screenOutput string) bool {
	clean := protocol.CleanTUIArtifacts(screenOutput)
	return strings.Contains(clean, "Antigravity") ||
		strings.Contains(clean, "Type a message")
}

func (a *AgyAgent) IsTurnFinished(screenOutput string, callsign string) bool {
	return protocol.HasTurnFinished(screenOutput, callsign)
}

func (a *AgyAgent) StopCommand() string {
	return "exit"
}

func (a *AgyAgent) ListModels() ([]string, error) {
	cmd := exec.Command("agy", "models")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var models []string
	lines := strings.Split(stdout.String(), "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			models = append(models, trimmed)
		}
	}
	return models, nil
}
