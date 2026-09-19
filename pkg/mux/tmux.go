package mux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type TmuxMux struct{}

func init() {
	Register(&TmuxMux{})
}

func (t *TmuxMux) Name() string {
	return "tmux"
}

func (t *TmuxMux) Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func (t *TmuxMux) DetectActive() bool {
	return os.Getenv("TMUX") != ""
}

func (t *TmuxMux) CreatePane(opts PaneOptions) (*PaneHandle, error) {
	if !t.Available() {
		return nil, fmt.Errorf("tmux is not installed")
	}

	splitFlag := "-h" // side-by-side
	if opts.Direction == "vertical" || opts.Direction == "down" {
		splitFlag = "-v"
	}

	size := 45
	if opts.Size > 0 {
		size = opts.Size
	}

	args := []string{"split-window", "-d", splitFlag, "-p", fmt.Sprintf("%d", size), "-P", "-F", "#{pane_id}"}
	if opts.Cwd != "" {
		args = append(args, "-c", opts.Cwd)
	}

	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tmux split-window failed: %w (output: %s)", err, string(out))
	}

	paneID := strings.TrimSpace(string(out))
	handle := &PaneHandle{
		MuxName: t.Name(),
		PaneID:  paneID,
	}

	if opts.Title != "" {
		_ = exec.Command("tmux", "select-pane", "-t", paneID, "-T", opts.Title).Run()
	}

	if len(opts.Command) > 0 {
		// Launch the command in the pane
		fullCmd := strings.Join(opts.Command, " ")
		if err := t.SendText(handle, fullCmd); err != nil {
			return nil, fmt.Errorf("failed to send launch command to tmux pane: %w", err)
		}
	}

	return handle, nil
}

func (t *TmuxMux) SendText(handle *PaneHandle, text string) error {
	if !t.Available() {
		return fmt.Errorf("tmux is not installed")
	}

	bufName := fmt.Sprintf("agent_pair_buf_%d", time.Now().UnixNano())

	// Load text into tmux buffer
	setBufCmd := exec.Command("tmux", "set-buffer", "-b", bufName, "--", text)
	if err := setBufCmd.Run(); err != nil {
		// Fallback: pipe to stdin of load-buffer
		loadCmd := exec.Command("tmux", "load-buffer", "-b", bufName, "-")
		loadCmd.Stdin = strings.NewReader(text)
		if err := loadCmd.Run(); err != nil {
			return fmt.Errorf("failed to set tmux buffer: %w", err)
		}
	}
	defer func() {
		_ = exec.Command("tmux", "delete-buffer", "-b", bufName).Run()
	}()

	// Paste buffer into the pane with -p (bracketed paste)
	pasteCmd := exec.Command("tmux", "paste-buffer", "-p", "-b", bufName, "-t", handle.PaneID)
	if out, err := pasteCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to paste buffer into tmux pane: %w (output: %s)", err, string(out))
	}

	// Send enter key to submit
	enterCmd := exec.Command("tmux", "send-keys", "-t", handle.PaneID, "C-m")
	if out, err := enterCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to send Enter key: %w (output: %s)", err, string(out))
	}

	return nil
}

func (t *TmuxMux) CaptureOutput(handle *PaneHandle) (string, error) {
	if !t.Available() {
		return "", fmt.Errorf("tmux is not installed")
	}

	cmd := exec.Command("tmux", "capture-pane", "-p", "-t", handle.PaneID, "-S", "-1000")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tmux capture-pane failed: %w", err)
	}

	return stdout.String(), nil
}

func (t *TmuxMux) ClosePane(handle *PaneHandle) error {
	if !t.Available() {
		return fmt.Errorf("tmux is not installed")
	}

	cmd := exec.Command("tmux", "kill-pane", "-t", handle.PaneID)
	_ = cmd.Run()
	return nil
}
