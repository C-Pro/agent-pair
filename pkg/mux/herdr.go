package mux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type HerdrMux struct{}

func init() {
	Register(&HerdrMux{})
}

func (h *HerdrMux) Name() string {
	return "herdr"
}

func (h *HerdrMux) Available() bool {
	_, err := exec.LookPath("herdr")
	return err == nil
}

func (h *HerdrMux) DetectActive() bool {
	if os.Getenv("HERDR_PANE_ID") != "" || os.Getenv("HERDR_SESSION") != "" {
		return true
	}
	out, err := exec.Command("herdr", "status", "server").CombinedOutput()
	if err == nil && strings.Contains(string(out), "running") && !strings.Contains(string(out), "not running") {
		return true
	}
	return false
}

func (h *HerdrMux) CreatePane(opts PaneOptions) (*PaneHandle, error) {
	if !h.Available() {
		return nil, fmt.Errorf("herdr is not installed")
	}

	direction := "right"
	if opts.Direction == "vertical" || opts.Direction == "down" {
		direction = "down"
	}

	args := []string{"pane", "split", "--direction", direction, "--no-focus"}
	if opts.Cwd != "" {
		args = append(args, "--cwd", opts.Cwd)
	}

	cmd := exec.Command("herdr", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("herdr pane split failed: %w (output: %s)", err, string(out))
	}

	paneID := strings.TrimSpace(string(out))
	handle := &PaneHandle{
		MuxName: h.Name(),
		PaneID:  paneID,
	}

	if len(opts.Command) > 0 {
		runArgs := append([]string{"pane", "run", paneID}, opts.Command...)
		if err := exec.Command("herdr", runArgs...).Run(); err != nil {
			return nil, fmt.Errorf("herdr pane run failed: %w", err)
		}
	}

	return handle, nil
}

func (h *HerdrMux) SendText(handle *PaneHandle, text string) error {
	if !h.Available() {
		return fmt.Errorf("herdr is not installed")
	}

	cmd := exec.Command("herdr", "pane", "send-text", handle.PaneID, text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("herdr pane send-text failed: %w (output: %s)", err, string(out))
	}

	keyCmd := exec.Command("herdr", "pane", "send-keys", handle.PaneID, "Enter")
	if out, err := keyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("herdr pane send-keys Enter failed: %w (output: %s)", err, string(out))
	}

	return nil
}

func (h *HerdrMux) CaptureOutput(handle *PaneHandle) (string, error) {
	if !h.Available() {
		return "", fmt.Errorf("herdr is not installed")
	}

	cmd := exec.Command("herdr", "pane", "read", handle.PaneID, "--format", "text", "--source", "recent")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("herdr pane read failed: %w", err)
	}

	return stdout.String(), nil
}

func (h *HerdrMux) ClosePane(handle *PaneHandle) error {
	if !h.Available() {
		return fmt.Errorf("herdr is not installed")
	}

	cmd := exec.Command("herdr", "pane", "close", handle.PaneID)
	_ = cmd.Run()
	return nil
}
