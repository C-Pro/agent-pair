package agent

import (
	"fmt"
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
		// read-only is an OS-level sandbox, not a prompt instruction: the
		// model's shell commands cannot write outside it.
		cmd = append(cmd, "-s", "read-only")
		// Escalation out of the sandbox needs a person. Without this the model
		// decides for itself when to ask.
		if supportsFlag("codex", "untrusted") {
			cmd = append(cmd, "-a", "untrusted")
		} else {
			cmd = append(cmd, "-a", "on-request")
		}
		if opts.Cwd != "" {
			// Pin the sandbox root instead of inheriting whatever directory
			// the pane happens to start in.
			cmd = append(cmd, "-C", opts.Cwd)
		}
	}

	return cmd, nil, nil
}

func (c *CodexAgent) IsReady(screenOutput string) bool {
	clean := protocol.CleanTUIArtifacts(screenOutput)
	if !strings.Contains(clean, "Codex") && !strings.Contains(clean, "OpenAI") {
		return false
	}

	// If the latest model status indicates it is still initializing, wait.
	if lastModelIdx := strings.LastIndex(clean, "model:"); lastModelIdx != -1 {
		modelLine := clean[lastModelIdx:]
		if endLine := strings.Index(modelLine, "\n"); endLine != -1 {
			modelLine = modelLine[:endLine]
		}
		if strings.Contains(modelLine, "loading") {
			return false
		}
	}

	// Folder trust prompt phrases across different Codex CLI / OS versions.
	trustPhrases := []string{
		"Do you trust the contents of this directory?",
		"Trust this folder?",
		"Trust and continue",
		"Press enter to continue",
		"1. Yes, continue",
	}

	lastTrustIdx := -1
	for _, phrase := range trustPhrases {
		if idx := strings.LastIndex(clean, phrase); idx > lastTrustIdx {
			lastTrustIdx = idx
		}
	}

	// If a trust prompt was displayed, ensure the user has answered it and
	// Codex has rendered an active input prompt after the trust dialog.
	if lastTrustIdx != -1 {
		lastPromptIdx := strings.LastIndex(clean, "Ask Codex to do anything")
		if lastPromptIdx <= lastTrustIdx {
			return false
		}
	}

	return true
}

func (c *CodexAgent) IsTurnFinished(screenOutput string, callsign string, turnID string) bool {
	return protocol.HasTurnFinished(screenOutput, callsign, turnID)
}

func (c *CodexAgent) StopCommand() string {
	return "exit"
}

// ListModels reports the models the installed Codex CLI knows about. Codex has
// no listing subcommand, so this falls back to the model ids its own --help
// documents rather than a hardcoded snapshot that goes stale.
func (c *CodexAgent) ListModels() ([]string, error) {
	return nil, fmt.Errorf("codex does not expose a model list; pass --model with a model id your Codex account is entitled to (see `codex --help` and your provider config)")
}
