package agent

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cpro/agent-pair/pkg/protocol"
)

type OpenCodeAgent struct{}

func init() {
	Register(&OpenCodeAgent{})
}

func (o *OpenCodeAgent) Name() string {
	return "opencode"
}

func (o *OpenCodeAgent) Available() bool {
	_, err := exec.LookPath("opencode")
	return err == nil
}

func (o *OpenCodeAgent) DefaultCallsign(model string) string {
	m := strings.ToLower(model)
	if strings.Contains(m, "muse") {
		return "muse"
	}
	if strings.Contains(m, "qwen") {
		return "qwen"
	}
	if strings.Contains(m, "gemma") {
		return "gemma"
	}
	if strings.Contains(m, "deepseek") {
		return "deepseek"
	}
	return "opencode"
}

func (o *OpenCodeAgent) BuildLaunchCommand(opts LaunchOptions) ([]string, map[string]string, error) {
	cmd := []string{"opencode"}

	// No default model: the follower reads the whole workspace, so which model
	// and which provider sees it has to be a deliberate choice.
	if opts.Model == "" {
		return nil, nil, fmt.Errorf("opencode follower needs an explicit --model (run `agent-pair models opencode`); pick one your organization has approved to receive this repository's contents")
	}
	cmd = append(cmd, "-m", opts.Model)

	if opts.ReadOnly {
		// Plan mode denies edit and write tools natively
		cmd = append(cmd, "--agent", "plan")
		// Load no third-party plugins: they add tools this session has not
		// vetted and paths for data to leave the machine.
		if supportsFlag("opencode", "--pure") {
			cmd = append(cmd, "--pure")
		}
	}

	return cmd, nil, nil
}

func (o *OpenCodeAgent) IsReady(screenOutput string) bool {
	clean := protocol.CleanTUIArtifacts(screenOutput)
	return strings.Contains(clean, "Ask anything") ||
		strings.Contains(clean, "OpenCode") ||
		strings.Contains(clean, "Plan ·") ||
		strings.Contains(clean, "Build ·")
}

func (o *OpenCodeAgent) IsTurnFinished(screenOutput string, callsign string, turnID string) bool {
	clean := protocol.CleanTUIArtifacts(screenOutput)

	// Must not be currently generating/thinking
	if strings.Contains(clean, "esc interrupt") || strings.Contains(clean, "Thinking") || strings.Contains(clean, "⠹") {
		return false
	}

	// Completion badge is present in OpenCode upon turn finish (e.g. ▣ Plan · Muse Spark 1.3 Free · 7.6s)
	if strings.Contains(clean, "▣") {
		return true
	}

	// Or if turn closing marker is present
	if protocol.HasTurnFinished(screenOutput, callsign, turnID) {
		return true
	}

	return false
}

func (o *OpenCodeAgent) StopCommand() string {
	return "exit"
}

func (o *OpenCodeAgent) ListModels() ([]string, error) {
	cmd := exec.Command("opencode", "models")
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
