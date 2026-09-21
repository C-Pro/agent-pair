package agent

import (
	"os/exec"
	"regexp"
	"sort"
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
		// Plan mode plus an explicit denial of the mutating tools.
		cmd = append(cmd, "--permission-mode", "plan", "--disallowed-tools", "Edit,Write,NotebookEdit,Bash")

		// --restricted drops every command- and code-running tool and WebFetch,
		// confines the file tools to the working directory, and ignores user
		// and project settings files, so a stray local setting cannot widen the
		// follower's permissions. Stronger than the denial list alone.
		if supportsFlag("claude", "--restricted") {
			cmd = append(cmd, "--restricted")
		}
		// Load no MCP servers: they are the follower's widest path to third
		// party tools and to data leaving this machine.
		if supportsFlag("claude", "--strict-mcp-config") {
			cmd = append(cmd, "--strict-mcp-config")
		}
	}

	return cmd, nil, nil
}

func (c *ClaudeAgent) IsReady(screenOutput string) bool {
	clean := protocol.CleanTUIArtifacts(screenOutput)
	return strings.Contains(clean, "Claude Code") ||
		strings.Contains(clean, "Welcome to Claude Code") ||
		// Fall back to the prompt box, which survives banner changes.
		strings.Contains(clean, "? for shortcuts")
}

func (c *ClaudeAgent) IsTurnFinished(screenOutput string, callsign string, turnID string) bool {
	return protocol.HasTurnFinished(screenOutput, callsign, turnID)
}

func (c *ClaudeAgent) StopCommand() string {
	return "/exit"
}

var claudeAliasRe = regexp.MustCompile(`'([a-z][a-z0-9.\-]*)'`)

// ListModels reports the model names the installed Claude Code accepts. Aliases
// are read out of the CLI's own --help text so the list tracks the installed
// binary rather than a hardcoded snapshot; an alias always resolves to the
// latest model behind it.
func (c *ClaudeAgent) ListModels() ([]string, error) {
	seen := map[string]bool{}
	for _, alias := range []string{"opus", "sonnet", "haiku"} {
		seen[alias] = true
	}

	out, err := exec.Command("claude", "--help").CombinedOutput()
	if err == nil {
		help := string(out)
		if idx := strings.Index(help, "--model <model>"); idx != -1 {
			block := help[idx:]
			if end := strings.Index(block, "\n  -"); end != -1 {
				block = block[:end]
			}
			for _, m := range claudeAliasRe.FindAllStringSubmatch(block, -1) {
				seen[m[1]] = true
			}
		}
	}

	models := make([]string, 0, len(seen))
	for m := range seen {
		models = append(models, m)
	}
	sort.Strings(models)
	return models, nil
}
