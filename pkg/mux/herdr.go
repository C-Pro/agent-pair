package mux

import (
	"bytes"
	"encoding/json"
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

	size := 45
	if opts.Size > 0 {
		size = opts.Size
	}

	// --ratio is the share of the split kept by the original (first) pane, so
	// the follower pane gets size percent.
	args := []string{"pane", "split", "--direction", direction, "--ratio", fmt.Sprintf("%.2f", float64(100-size)/100), "--no-focus"}
	// Split the pane agent-pair was started from (the leader's pane). Without
	// a pane, herdr may split the UI-focused pane, which can be in another tab.
	if leaderPane := os.Getenv("HERDR_PANE_ID"); leaderPane != "" {
		// HERDR_PANE_ID can name a pane that no longer exists, for example
		// when the environment was inherited. Targeting it would fail the
		// split, so fall back to herdr's default target.
		if alive, _ := h.PaneAlive(&PaneHandle{PaneID: leaderPane}); alive {
			args = append(args, "--pane", leaderPane)
		} else {
			fmt.Fprintf(os.Stderr, "warning: HERDR_PANE_ID=%s is not a live pane; splitting the focused pane instead\n", leaderPane)
		}
	}
	if opts.Cwd != "" {
		args = append(args, "--cwd", opts.Cwd)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("herdr", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("herdr pane split failed: %w (output: %s%s)", err, stdout.String(), stderr.String())
	}

	paneID, err := parseHerdrSplitPaneID(stdout.Bytes())
	if err != nil {
		return nil, err
	}
	handle := &PaneHandle{
		MuxName: h.Name(),
		PaneID:  paneID,
	}

	if opts.Title != "" {
		_ = exec.Command("herdr", "pane", "rename", paneID, opts.Title).Run()
	}

	if len(opts.Command) > 0 {
		// herdr types the command into the pane's shell, so every argument
		// must be quoted.
		runCmd := exec.Command("herdr", "pane", "run", paneID, shellCommand(opts.Command))
		if out, err := runCmd.CombinedOutput(); err != nil {
			_ = h.ClosePane(handle)
			return nil, fmt.Errorf("herdr pane run failed: %w (output: %s)", err, string(out))
		}
	}

	return handle, nil
}

// parseHerdrSplitPaneID reads the new pane id from the JSON response of
// `herdr pane split`.
func parseHerdrSplitPaneID(out []byte) (string, error) {
	var resp struct {
		Result struct {
			Pane struct {
				PaneID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil || resp.Result.Pane.PaneID == "" {
		return "", fmt.Errorf("herdr pane split did not report a pane id (output: %s)", strings.TrimSpace(string(out)))
	}
	return resp.Result.Pane.PaneID, nil
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

func (h *HerdrMux) PaneAlive(handle *PaneHandle) (bool, error) {
	if !h.Available() {
		return false, fmt.Errorf("herdr is not installed")
	}
	if handle == nil || handle.PaneID == "" {
		return false, nil
	}

	cmd := exec.Command("herdr", "pane", "read", handle.PaneID, "--format", "text", "--source", "recent")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
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
	if handle == nil || handle.PaneID == "" {
		return nil
	}
	alive, err := h.PaneAlive(handle)
	if err != nil {
		return err
	}
	if !alive {
		return nil
	}

	cmd := exec.Command("herdr", "pane", "close", handle.PaneID)
	if out, err := cmd.CombinedOutput(); err != nil {
		if stillAlive, checkErr := h.PaneAlive(handle); checkErr == nil && !stillAlive {
			return nil
		}
		return fmt.Errorf("herdr pane close failed: %w (output: %s)", err, string(out))
	}
	return nil
}
