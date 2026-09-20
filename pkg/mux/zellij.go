package mux

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

type ZellijMux struct{}

func init() {
	Register(&ZellijMux{})
}

func (z *ZellijMux) Name() string {
	return "zellij"
}

func (z *ZellijMux) Available() bool {
	_, err := exec.LookPath("zellij")
	return err == nil
}

func (z *ZellijMux) DetectActive() bool {
	return os.Getenv("ZELLIJ") != ""
}

var zellijPaneRegex = regexp.MustCompile(`(terminal_\d+|\d+)`)

func (z *ZellijMux) CreatePane(opts PaneOptions) (*PaneHandle, error) {
	if !z.Available() {
		return nil, fmt.Errorf("zellij is not installed")
	}

	direction := "right"
	if opts.Direction == "vertical" || opts.Direction == "down" {
		direction = "down"
	}

	args := []string{"run"}
	if opts.Title != "" {
		args = append(args, "--name", opts.Title)
	}
	if opts.Cwd != "" {
		args = append(args, "--cwd", opts.Cwd)
	}
	args = append(args, "--direction", direction, "--no-focus", "--")

	if len(opts.Command) > 0 {
		args = append(args, opts.Command...)
	} else {
		args = append(args, "bash", "-l")
	}

	cmd := exec.Command("zellij", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("zellij run failed: %w (output: %s)", err, string(out))
	}

	match := zellijPaneRegex.FindString(string(out))
	if match == "" {
		// Fallback: if pane id not printed, use title or dump active
		match = opts.Title
	}

	return &PaneHandle{
		MuxName: z.Name(),
		PaneID:  match,
	}, nil
}

func (z *ZellijMux) SendText(handle *PaneHandle, text string) error {
	if !z.Available() {
		return fmt.Errorf("zellij is not installed")
	}

	// Write characters to the pane
	args := []string{"action", "write-chars"}
	if handle.PaneID != "" {
		args = append(args, "-p", handle.PaneID)
	}
	args = append(args, text)

	writeCmd := exec.Command("zellij", args...)
	if out, err := writeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zellij write-chars failed: %w (output: %s)", err, string(out))
	}

	// Send enter key
	keyArgs := []string{"action", "send-keys"}
	if handle.PaneID != "" {
		keyArgs = append(keyArgs, "-p", handle.PaneID)
	}
	keyArgs = append(keyArgs, "Enter")

	enterCmd := exec.Command("zellij", keyArgs...)
	if out, err := enterCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zellij send-keys Enter failed: %w (output: %s)", err, string(out))
	}

	return nil
}

func (z *ZellijMux) PaneAlive(handle *PaneHandle) (bool, error) {
	if !z.Available() {
		return false, fmt.Errorf("zellij is not installed")
	}
	if handle == nil || handle.PaneID == "" {
		return false, nil
	}

	cmd := exec.Command("zellij", "action", "dump-screen", "-f", "-p", handle.PaneID)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (z *ZellijMux) CaptureOutput(handle *PaneHandle) (string, error) {
	if !z.Available() {
		return "", fmt.Errorf("zellij is not installed")
	}

	args := []string{"action", "dump-screen", "-f"}
	if handle.PaneID != "" {
		args = append(args, "-p", handle.PaneID)
	}

	cmd := exec.Command("zellij", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("zellij dump-screen failed: %w", err)
	}

	return stdout.String(), nil
}

func (z *ZellijMux) ClosePane(handle *PaneHandle) error {
	if !z.Available() {
		return fmt.Errorf("zellij is not installed")
	}
	if handle == nil || handle.PaneID == "" {
		return fmt.Errorf("cannot close zellij pane without a pane ID")
	}

	cmd := exec.Command("zellij", "action", "close-pane", "-p", handle.PaneID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("zellij close-pane failed: %w (output: %s)", err, string(out))
	}
	return nil
}
