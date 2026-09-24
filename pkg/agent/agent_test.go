package agent

import (
	"os"
	"strings"
	"testing"
)

func TestDeriveCallsign(t *testing.T) {
	tests := []struct {
		model    string
		fallback string
		expected string
	}{
		{
			model:    "opencode/muse-spark-1.3-contributor-free",
			fallback: "muse",
			expected: "muse-spark-1.3",
		},
		{
			model:    "opencode/big-pickle",
			fallback: "opencode",
			expected: "big-pickle",
		},
		{
			model:    "Lemonade/Qwen3.6-35B-A3B-MTP-GGUF",
			fallback: "qwen",
			expected: "qwen3.6-35b-a3b",
		},
		{
			model:    "ollama/gemma4:26b",
			fallback: "gemma",
			expected: "gemma4-26b",
		},
		{
			model:    "claude-3-7-sonnet-latest",
			fallback: "claude",
			expected: "claude-3-7-sonnet",
		},
		{
			model:    "gemini-3.8-flash",
			fallback: "gemini",
			expected: "gemini-3.8-flash",
		},
		{
			model:    "o3-mini",
			fallback: "codex",
			expected: "o3-mini",
		},
		{
			model:    "model_with_underscores and spaces",
			fallback: "default",
			expected: "model-with-underscores-and-spaces",
		},
		{
			model:    "---extra--hyphens---",
			fallback: "default",
			expected: "extra-hyphens",
		},
		{
			model:    "",
			fallback: "gemini",
			expected: "gemini",
		},
		{
			model:    "////",
			fallback: "fallback-agent",
			expected: "fallback-agent",
		},
	}

	for _, tt := range tests {
		got := DeriveCallsign(tt.model, tt.fallback)
		if got != tt.expected {
			t.Errorf("DeriveCallsign(%q, %q) = %q; want %q", tt.model, tt.fallback, got, tt.expected)
		}
	}
}

func TestRegistry(t *testing.T) {
	list := List()
	expected := map[string]bool{
		"opencode": false,
		"agy":      false,
		"claude":   false,
		"codex":    false,
	}
	for _, name := range list {
		expected[name] = true
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected registered agent %q in List()", name)
		}
	}

	_, err := Get("unknown-agent-xyz")
	if err == nil || !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("expected 'unknown agent' error, got %v", err)
	}

	// Test when agent binary is not available on PATH
	t.Run("agent unavailable", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, err := Get("claude")
		if err == nil || !strings.Contains(err.Error(), "not installed or available on PATH") {
			t.Errorf("expected 'not installed or available' error, got %v", err)
		}
	})
}

func TestSanitizeLaunchCommand(t *testing.T) {
	rawCmd := []string{"opencode", "-m", "model"}
	sanitized := SanitizeLaunchCommand(rawCmd, map[string]string{"B": "2", "A": "1"})

	if len(sanitized) <= len(rawCmd) {
		t.Fatalf("expected sanitized command to have env unsets")
	}

	if sanitized[0] != "env" {
		t.Errorf("expected first element to be env, got %q", sanitized[0])
	}

	envAt := len(sanitized) - len(rawCmd) - 2
	if got := sanitized[envAt : envAt+2]; got[0] != "A=1" || got[1] != "B=2" {
		t.Errorf("expected sorted env assignments before the command, got %q", got)
	}

	lastElements := sanitized[len(sanitized)-len(rawCmd):]
	for i, v := range rawCmd {
		if lastElements[i] != v {
			t.Errorf("expected suffix %q, got %q", v, lastElements[i])
		}
	}
}

func TestOpenCodeAgent(t *testing.T) {
	oa := &OpenCodeAgent{}

	if oa.Name() != "opencode" {
		t.Errorf("Name() = %q, want 'opencode'", oa.Name())
	}
	if oa.StopCommand() != "exit" {
		t.Errorf("StopCommand() = %q, want 'exit'", oa.StopCommand())
	}

	// DefaultCallsign
	if oa.DefaultCallsign("muse-model") != "muse" {
		t.Errorf("DefaultCallsign(muse) failed")
	}
	if oa.DefaultCallsign("qwen-coder") != "qwen" {
		t.Errorf("DefaultCallsign(qwen) failed")
	}
	if oa.DefaultCallsign("gemma-2b") != "gemma" {
		t.Errorf("DefaultCallsign(gemma) failed")
	}
	if oa.DefaultCallsign("deepseek-chat") != "deepseek" {
		t.Errorf("DefaultCallsign(deepseek) failed")
	}
	if oa.DefaultCallsign("other-model") != "opencode" {
		t.Errorf("DefaultCallsign(other) failed")
	}

	// A follower reads the whole workspace, so the model that receives it must
	// be chosen deliberately rather than defaulted to.
	if _, _, err := oa.BuildLaunchCommand(LaunchOptions{ReadOnly: true}); err == nil {
		t.Fatal("expected BuildLaunchCommand to require an explicit model")
	}

	cmd, _, err := oa.BuildLaunchCommand(LaunchOptions{Model: "vendor/model", ReadOnly: true})
	if err != nil {
		t.Fatalf("BuildLaunchCommand failed: %v", err)
	}
	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "-m vendor/model") {
		t.Errorf("expected the requested model, got %v", cmdStr)
	}
	if !strings.Contains(cmdStr, "--agent plan") {
		t.Errorf("expected --agent plan for ReadOnly")
	}

	// BuildLaunchCommand custom model & read-only false
	cmd2, _, err := oa.BuildLaunchCommand(LaunchOptions{Model: "custom/model", ReadOnly: false})
	if err != nil {
		t.Fatalf("BuildLaunchCommand failed: %v", err)
	}
	cmdStr2 := strings.Join(cmd2, " ")
	if !strings.Contains(cmdStr2, "-m custom/model") {
		t.Errorf("expected custom model, got %v", cmdStr2)
	}
	if strings.Contains(cmdStr2, "--agent plan") {
		t.Errorf("did not expect --agent plan when ReadOnly is false")
	}

	// IsReady
	if !oa.IsReady("Welcome! Ask anything") {
		t.Errorf("IsReady failed for Ask anything")
	}
	if !oa.IsReady("OpenCode v1.0") {
		t.Errorf("IsReady failed for OpenCode")
	}
	if !oa.IsReady("Plan · mode active") {
		t.Errorf("IsReady failed for Plan ·")
	}
	if !oa.IsReady("Build · mode active") {
		t.Errorf("IsReady failed for Build ·")
	}
	if oa.IsReady("Random uninitialized text") {
		t.Errorf("IsReady should return false for uninitialized text")
	}

	// IsTurnFinished
	if oa.IsTurnFinished("esc interrupt to cancel", "muse", "") {
		t.Errorf("IsTurnFinished should be false during esc interrupt")
	}
	if oa.IsTurnFinished("Thinking...", "muse", "") {
		t.Errorf("IsTurnFinished should be false during Thinking")
	}
	if oa.IsTurnFinished("Loading ⠹ spinner", "muse", "") {
		t.Errorf("IsTurnFinished should be false during spinner")
	}
	if !oa.IsTurnFinished("▣ Plan completed", "muse", "") {
		t.Errorf("IsTurnFinished should be true for ▣ badge")
	}
	if !oa.IsTurnFinished("[ muse over ]", "muse", "") {
		t.Errorf("IsTurnFinished should be true for muse over")
	}
	if oa.IsTurnFinished("Incomplete response", "muse", "") {
		t.Errorf("IsTurnFinished should be false for incomplete response")
	}
}

func TestAgyAgent(t *testing.T) {
	aa := &AgyAgent{}

	if aa.Name() != "agy" {
		t.Errorf("Name() = %q, want 'agy'", aa.Name())
	}
	if aa.StopCommand() != "exit" {
		t.Errorf("StopCommand() = %q, want 'exit'", aa.StopCommand())
	}

	// DefaultCallsign
	if aa.DefaultCallsign("claude-sonnet") != "claude" {
		t.Errorf("DefaultCallsign(claude) failed")
	}
	if aa.DefaultCallsign("gpt-4o") != "gpt" {
		t.Errorf("DefaultCallsign(gpt) failed")
	}
	if aa.DefaultCallsign("gemini-1.5") != "gemini" {
		t.Errorf("DefaultCallsign(other) failed")
	}

	// BuildLaunchCommand
	cmd, _, err := aa.BuildLaunchCommand(LaunchOptions{Model: "gemini-pro", Effort: "high", ReadOnly: true})
	if err != nil {
		t.Fatalf("BuildLaunchCommand failed: %v", err)
	}
	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--model gemini-pro") || !strings.Contains(cmdStr, "--effort high") || !strings.Contains(cmdStr, "--mode plan --sandbox") {
		t.Errorf("unexpected command: %v", cmdStr)
	}
	if _, _, err := aa.BuildLaunchCommand(LaunchOptions{Effort: "extreme"}); err == nil {
		t.Fatal("expected invalid effort to fail")
	}

	cmd2, _, _ := aa.BuildLaunchCommand(LaunchOptions{})
	if strings.Contains(strings.Join(cmd2, " "), "--mode plan") {
		t.Errorf("did not expect plan mode")
	}

	// IsReady
	if !aa.IsReady("Antigravity CLI") || !aa.IsReady("Type a message") {
		t.Errorf("IsReady failed for agy")
	}
	if aa.IsReady("agy --model gemini") || aa.IsReady("other") {
		t.Errorf("IsReady should return false for other text")
	}

	// IsTurnFinished
	if !aa.IsTurnFinished("[ gemini over ]", "gemini", "") {
		t.Errorf("IsTurnFinished failed for gemini")
	}
}

func TestClaudeAgent(t *testing.T) {
	ca := &ClaudeAgent{}

	if ca.Name() != "claude" {
		t.Errorf("Name() = %q, want 'claude'", ca.Name())
	}
	if ca.DefaultCallsign("any") != "claude" {
		t.Errorf("DefaultCallsign = %q, want 'claude'", ca.DefaultCallsign("any"))
	}
	if ca.StopCommand() != "/exit" {
		t.Errorf("StopCommand() = %q, want '/exit'", ca.StopCommand())
	}

	// BuildLaunchCommand
	cmd, env, err := ca.BuildLaunchCommand(LaunchOptions{Model: "opus", ReadOnly: true})
	if err != nil {
		t.Fatalf("BuildLaunchCommand failed: %v", err)
	}
	if env["CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN"] != "1" {
		t.Errorf("expected the alternate screen to be disabled so replies stay in scrollback, got env %v", env)
	}
	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--model opus") {
		t.Errorf("unexpected command: %v", cmdStr)
	}
	if !strings.Contains(cmdStr, "--permission-mode plan --disallowed-tools Edit,Write,NotebookEdit,Bash") {
		t.Errorf("expected the mutating tools to be denied, got: %v", cmdStr)
	}

	cmd2, _, err := ca.BuildLaunchCommand(LaunchOptions{Model: "opus", ReadOnly: false})
	if err != nil {
		t.Fatalf("BuildLaunchCommand failed: %v", err)
	}
	if strings.Contains(strings.Join(cmd2, " "), "--permission-mode") {
		t.Errorf("did not expect read-only hardening when ReadOnly is false")
	}

	// IsReady
	if !ca.IsReady("Welcome to Claude Code") || !ca.IsReady("Claude Code") {
		t.Errorf("IsReady failed for claude")
	}
	if ca.IsReady("other prompt") {
		t.Errorf("IsReady should return false")
	}

	// IsTurnFinished
	if !ca.IsTurnFinished("[ claude over ]", "claude", "") {
		t.Errorf("IsTurnFinished failed")
	}

	// ListModels
	models, err := ca.ListModels()
	if err != nil || len(models) == 0 {
		t.Fatalf("ListModels failed: %v", err)
	}
}

func TestCodexAgent(t *testing.T) {
	cda := &CodexAgent{}

	if cda.Name() != "codex" {
		t.Errorf("Name() = %q, want 'codex'", cda.Name())
	}
	if cda.DefaultCallsign("any") != "codex" {
		t.Errorf("DefaultCallsign = %q, want 'codex'", cda.DefaultCallsign("any"))
	}
	if cda.StopCommand() != "exit" {
		t.Errorf("StopCommand() = %q, want 'exit'", cda.StopCommand())
	}

	// BuildLaunchCommand
	cmd, _, err := cda.BuildLaunchCommand(LaunchOptions{Model: "o3", ReadOnly: true})
	if err != nil {
		t.Fatalf("BuildLaunchCommand failed: %v", err)
	}
	cmdStr := strings.Join(cmd, " ")
	if !strings.Contains(cmdStr, "--no-alt-screen") || !strings.Contains(cmdStr, "-m o3") || !strings.Contains(cmdStr, "-s read-only") {
		t.Errorf("unexpected command: %v", cmdStr)
	}
	if !strings.Contains(cmdStr, "-a untrusted") {
		t.Errorf("expected sandbox escalation to require approval, got: %v", cmdStr)
	}

	// IsReady
	if !cda.IsReady("Codex CLI") || !cda.IsReady("OpenAI") {
		t.Errorf("IsReady failed for codex")
	}
	if cda.IsReady("other") {
		t.Errorf("IsReady should return false")
	}

	// IsTurnFinished
	if !cda.IsTurnFinished("[ codex over ]", "codex", "") {
		t.Errorf("IsTurnFinished failed")
	}

	// Codex has no listing command; saying so beats returning a stale snapshot.
	if _, err := cda.ListModels(); err == nil {
		t.Fatal("expected ListModels to report that codex cannot list models")
	}
}

func TestAgentAvailableMethods(t *testing.T) {
	for _, name := range []string{"opencode", "agy", "claude", "codex"} {
		// Just ensure Available() executes without panic
		if a, ok := registry[name]; ok {
			_ = a.Available()
		}
	}

	// In an empty PATH environment, all should return false
	t.Setenv("PATH", t.TempDir())
	for _, name := range []string{"opencode", "agy", "claude", "codex"} {
		if a, ok := registry[name]; ok {
			if a.Available() {
				t.Errorf("expected Available() = false for %s in empty PATH", name)
			}
		}
	}
}

func TestOpenCodeListModels(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("skipping in CI: agent binary for opencode is missing")
	}
	oa := &OpenCodeAgent{}
	if !oa.Available() {
		t.Skip("opencode binary not available")
	}
	models, err := oa.ListModels()
	if err != nil {
		t.Skipf("opencode models is unavailable in this environment: %v", err)
	}
	if len(models) == 0 {
		t.Errorf("expected non-empty models list")
	}
}

func TestClaudeReadOnlyUsesAvailableHardening(t *testing.T) {
	ca := &ClaudeAgent{}
	if !ca.Available() {
		t.Skip("claude binary not available")
	}

	cmd, _, err := ca.BuildLaunchCommand(LaunchOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("BuildLaunchCommand failed: %v", err)
	}
	cmdStr := strings.Join(cmd, " ")

	// The flags are added only when the installed CLI advertises them, so the
	// expectation follows the binary rather than a pinned version.
	for _, flag := range []string{"--restricted", "--strict-mcp-config"} {
		want := supportsFlag("claude", flag)
		if got := strings.Contains(cmdStr, flag); got != want {
			t.Errorf("%s present = %v, want %v (command: %s)", flag, got, want, cmdStr)
		}
	}
}

func TestClaudeListModelsReportsAliasesNotSnapshots(t *testing.T) {
	ca := &ClaudeAgent{}
	models, err := ca.ListModels()
	if err != nil {
		t.Fatalf("ListModels failed: %v", err)
	}

	seen := map[string]bool{}
	for _, m := range models {
		seen[m] = true
		// A dated model id pins a snapshot that goes stale; aliases do not.
		if strings.HasPrefix(m, "claude-3-") {
			t.Errorf("ListModels returned the superseded snapshot %q", m)
		}
	}
	for _, alias := range []string{"opus", "sonnet", "haiku"} {
		if !seen[alias] {
			t.Errorf("expected alias %q in %v", alias, models)
		}
	}
}
