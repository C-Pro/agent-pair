package agent

import (
	"bytes"
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

	model := opts.Model
	if model == "" {
		model = "opencode/muse-spark-1.3-contributor-free"
	}
	cmd = append(cmd, "-m", model)

	if opts.ReadOnly {
		// Plan mode denies edit and write tools natively
		cmd = append(cmd, "--agent", "plan")
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

func (o *OpenCodeAgent) IsTurnFinished(screenOutput string, callsign string) bool {
	clean := protocol.CleanTUIArtifacts(screenOutput)

	// Must have the closing marker
	if !protocol.HasTurnFinished(screenOutput, callsign) {
		return false
	}

	// Must not be currently generating/thinking
	if strings.Contains(clean, "esc interrupt") || strings.Contains(clean, "Thinking") {
		return false
	}

	// Completion indicator in OpenCode
	return strings.Contains(clean, "▣") || strings.Contains(clean, "OpenCode") || strings.Contains(clean, "Ask anything")
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
