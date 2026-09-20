package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cpro/agent-pair/pkg/agent"
	"github.com/cpro/agent-pair/pkg/mux"
	"github.com/cpro/agent-pair/pkg/protocol"
	"github.com/cpro/agent-pair/pkg/session"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "start":
		err = runStart(args)
	case "send":
		err = runSend(args)
	case "wait":
		err = runWait(args)
	case "turn":
		err = runTurn(args)
	case "stop":
		err = runStop(args)
	case "status":
		err = runStatus(args)
	case "models":
		err = runModels(args)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`agent-pair: Multi-Agent Pair Programming CLI

USAGE:
  agent-pair <command> [arguments...]

COMMANDS:
  start     Initialize a new pair programming session in a demuxer pane
  send      Send a turn message to the peer agent
  wait      Wait for and extract the peer agent's response
  turn      Send a message and wait for the response in one step
  status    Show active session information
  stop      Gracefully shut down the peer agent and close the pane
  models    List supported models for an agent

MULTIPLEXERS SUPPORTED:
  tmux, zellij, herdr (auto-detected from environment)

AGENTS SUPPORTED:
  opencode, agy, claude, codex

EXAMPLES:
  # Start a session with opencode using default muse model in tmux/zellij/herdr
  agent-pair start --follower opencode --model opencode/muse-spark-1.3-contributor-free

  # Send a turn and wait for reply
  agent-pair turn "What are the tradeoffs of using Go channels vs mutexes here?"

  # Stop session and close pane
  agent-pair stop`)
}

func runStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ExitOnError)

	muxName := fs.String("mux", "auto", "Multiplexer to use (auto, tmux, zellij, herdr)")
	followerName := fs.String("follower", "opencode", "Follower agent (opencode, agy, claude, codex)")
	leaderName := fs.String("leader", "agy", "Leader agent (agy, opencode, claude, codex)")
	model := fs.String("model", "", "Model identifier for follower")
	leaderModelFlag := fs.String("leader-model", "", "Model identifier for leader")
	followerCallsign := fs.String("follower-callsign", "", "Explicit callsign for follower (defaults to model name)")
	leaderCallsign := fs.String("leader-callsign", "", "Explicit callsign for leader (defaults to model name)")
	readOnly := fs.Bool("read-only", true, "Start follower in read-only mode")
	cwd := fs.String("cwd", "", "Working directory (defaults to current dir)")
	direction := fs.String("direction", "horizontal", "Pane split direction (horizontal, vertical)")
	size := fs.Int("size", 45, "Pane size percentage")
	timeoutSec := fs.Int("timeout", 45, "Timeout in seconds to wait for agent initialization")
	noBootstrap := fs.Bool("no-bootstrap", false, "Skip initial radio protocol handshake")
	force := fs.Bool("force", false, "Overwrite existing active session")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if session.Exists() && !*force {
		return fmt.Errorf("an active session already exists (use 'agent-pair status', 'agent-pair stop', or --force)")
	}

	// Resolve working directory
	targetCwd := *cwd
	if targetCwd == "" {
		var err error
		targetCwd, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	// Resolve multiplexer
	m, err := mux.DetectOrGet(*muxName)
	if err != nil {
		return err
	}

	// Resolve follower agent adapter
	followerAdapter, err := agent.Get(*followerName)
	if err != nil {
		return err
	}

	// Resolve leader agent adapter
	leaderAdapter, err := agent.Get(*leaderName)
	if err != nil {
		return err
	}

	// Resolve model
	targetModel := *model
	if targetModel == "" && followerAdapter.Name() == "opencode" {
		targetModel = "opencode/muse-spark-1.3-contributor-free"
	}

	// Resolve callsigns: always follow model names unless explicitly overridden
	fCallsign := *followerCallsign
	if fCallsign == "" {
		fCallsign = agent.DeriveCallsign(targetModel, followerAdapter.Name())
	}

	lCallsign := *leaderCallsign
	if lCallsign == "" {
		lModel := *leaderModelFlag
		if lModel == "" && leaderAdapter.Name() == "agy" {
			lModel = os.Getenv("GEMINI_MODEL")
		}
		lCallsign = agent.DeriveCallsign(lModel, leaderAdapter.DefaultCallsign(""))
	}

	// Build follower launch command
	cmd, _, err := followerAdapter.BuildLaunchCommand(agent.LaunchOptions{
		Model:    targetModel,
		ReadOnly: *readOnly,
		Cwd:      targetCwd,
		Callsign: fCallsign,
	})
	if err != nil {
		return fmt.Errorf("failed to build launch command: %w", err)
	}
	cmd = agent.SanitizeLaunchCommand(cmd)

	title := fmt.Sprintf("[pair:%s]", fCallsign)
	fmt.Printf("==> Launching %s (%s) in %s pane...\n", followerAdapter.Name(), fCallsign, m.Name())

	// Create pane and launch follower
	handle, err := m.CreatePane(mux.PaneOptions{
		Title:     title,
		Cwd:       targetCwd,
		Command:   cmd,
		Direction: *direction,
		Size:      *size,
	})
	if err != nil {
		return fmt.Errorf("failed to create pane: %w", err)
	}

	// Wait for agent to become ready
	fmt.Printf("==> Waiting for %s to initialize (timeout %ds)...\n", fCallsign, *timeoutSec)
	ready := false
	startTime := time.Now()
	timeout := time.Duration(*timeoutSec) * time.Second

	for time.Since(startTime) < timeout {
		time.Sleep(1 * time.Second)
		output, err := m.CaptureOutput(handle)
		if err != nil {
			continue
		}
		if followerAdapter.IsReady(output) {
			ready = true
			break
		}
	}

	if !ready {
		_ = m.ClosePane(handle)
		return fmt.Errorf("agent %s failed to reach ready state within %ds", fCallsign, *timeoutSec)
	}

	fmt.Printf("==> %s is ready!\n", fCallsign)

	// Save session early so subsequent calls know the handle
	sess := &session.Session{
		ID:               fmt.Sprintf("pair-%d", time.Now().Unix()),
		MuxName:          m.Name(),
		PaneHandle:       handle,
		LeaderAgent:      leaderAdapter.Name(),
		LeaderCallsign:   lCallsign,
		FollowerAgent:    followerAdapter.Name(),
		FollowerCallsign: fCallsign,
		FollowerModel:    targetModel,
		Cwd:              targetCwd,
		ReadOnly:         *readOnly,
		CreatedAt:        time.Now(),
	}
	if err := session.Save(sess); err != nil {
		return fmt.Errorf("failed to save session state: %w", err)
	}

	// Initial bootstrap handshake
	if !*noBootstrap {
		fmt.Printf("==> Transmitting protocol handshake to %s...\n", fCallsign)
		baseline := 0
		if output, captureErr := m.CaptureOutput(handle); captureErr == nil {
			baseline = protocol.CountTurnEndMarkers(output, fCallsign)
		}
		bootstrap := protocol.FormatBootstrapPrompt(lCallsign, fCallsign, targetCwd, *readOnly)
		if err := m.SendText(handle, bootstrap); err != nil {
			return fmt.Errorf("failed to send bootstrap prompt: %w", err)
		}

		fmt.Printf("==> Awaiting protocol acknowledgment...\n")
		ackDone := false
		ackStart := time.Now()
		for time.Since(ackStart) < timeout {
			time.Sleep(1 * time.Second)
			output, err := m.CaptureOutput(handle)
			if err != nil {
				continue
			}
			if protocol.CountTurnEndMarkers(output, fCallsign) > baseline && followerAdapter.IsTurnFinished(output, fCallsign) {
				ackDone = true
				if body, found := protocol.ExtractLatestTurn(output, fCallsign, lCallsign); found {
					fmt.Printf("\n[ %s ACK RECEIVED ]\n%s\n\n", fCallsign, body)
				}
				break
			}
		}

		if !ackDone {
			fmt.Printf("Warning: ACK marker not detected yet, but session is initialized.\n")
		}
	}

	fmt.Printf("==> Pair programming session established!\n")
	fmt.Printf("    Leader:   %s (%s)\n", leaderAdapter.Name(), lCallsign)
	fmt.Printf("    Follower: %s (%s, model: %s)\n", followerAdapter.Name(), fCallsign, targetModel)
	fmt.Printf("    Channel:  %s (pane: %s)\n", m.Name(), handle.PaneID)
	return nil
}

func readMessageFromArgsOrStdin(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}

	// Check if stdin has data
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}

	return "", fmt.Errorf("no message provided (pass as argument or via stdin)")
}

func runSend(args []string) error {
	msg, err := readMessageFromArgsOrStdin(args)
	if err != nil {
		return err
	}

	sess, err := session.Load()
	if err != nil {
		return err
	}

	m, err := mux.Get(sess.MuxName)
	if err != nil {
		return err
	}

	output, err := m.CaptureOutput(sess.PaneHandle)
	if err != nil {
		return fmt.Errorf("failed to capture follower output before sending turn: %w", err)
	}
	sess.ResponseBaseline = protocol.CountTurnEndMarkers(output, sess.FollowerCallsign)
	if err := session.Save(sess); err != nil {
		return fmt.Errorf("failed to save response watermark: %w", err)
	}

	formatted := protocol.FormatTurn(sess.LeaderCallsign, sess.FollowerCallsign, msg)
	if err := m.SendText(sess.PaneHandle, formatted); err != nil {
		return fmt.Errorf("failed to send turn message: %w", err)
	}

	fmt.Printf("==> Turn sent to %s [%s]\n", sess.FollowerCallsign, sess.PaneHandle.PaneID)
	return nil
}

func runWait(args []string) error {
	fs := flag.NewFlagSet("wait", flag.ExitOnError)
	timeoutSec := fs.Int("timeout", 120, "Timeout in seconds to wait for peer response")
	raw := fs.Bool("raw", false, "Print raw uncleaned screen output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	sess, err := session.Load()
	if err != nil {
		return err
	}

	m, err := mux.Get(sess.MuxName)
	if err != nil {
		return err
	}

	followerAdapter, err := agent.Get(sess.FollowerAgent)
	if err != nil {
		return err
	}

	timeout := time.Duration(*timeoutSec) * time.Second
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		time.Sleep(1 * time.Second)

		output, err := m.CaptureOutput(sess.PaneHandle)
		if err != nil {
			continue
		}

		if protocol.CountTurnEndMarkers(output, sess.FollowerCallsign) > sess.ResponseBaseline && followerAdapter.IsTurnFinished(output, sess.FollowerCallsign) {
			if *raw {
				fmt.Println(output)
				return nil
			}

			if body, found := protocol.ExtractLatestTurn(output, sess.FollowerCallsign, sess.LeaderCallsign); found {
				fmt.Printf("[ CQ %s -> %s ]\n\n%s\n\n[ %s over ]\n",
					sess.FollowerCallsign, sess.LeaderCallsign, body, sess.FollowerCallsign)
				return nil
			}

			// Fallback: output cleaned screen text
			cleaned := protocol.CleanTUIArtifacts(output)
			fmt.Println(cleaned)
			return nil
		}
	}

	return fmt.Errorf("timeout (%ds) waiting for %s to complete response", *timeoutSec, sess.FollowerCallsign)
}

func runTurn(args []string) error {
	msg, err := readMessageFromArgsOrStdin(args)
	if err != nil {
		return err
	}

	// Send message
	if err := runSend([]string{msg}); err != nil {
		return err
	}

	// Wait for response
	return runWait([]string{})
}

func runStop(args []string) error {
	sess, err := session.Load()
	if err != nil {
		return err
	}

	m, err := mux.Get(sess.MuxName)
	if err != nil {
		return err
	}

	followerAdapter, err := agent.Get(sess.FollowerAgent)
	if err != nil {
		// Fallback to exit
		_ = m.SendText(sess.PaneHandle, "exit")
	} else {
		_ = m.SendText(sess.PaneHandle, followerAdapter.StopCommand())
	}

	time.Sleep(1 * time.Second)
	_ = m.ClosePane(sess.PaneHandle)
	_ = session.Clear()

	fmt.Printf("==> Pair programming session closed cleanly.\n")
	return nil
}

func runStatus(args []string) error {
	sess, err := session.Load()
	if err != nil {
		return err
	}

	m, err := mux.Get(sess.MuxName)
	if err != nil {
		return err
	}

	uptime := time.Since(sess.CreatedAt).Round(time.Second)

	fmt.Printf("Active Pair Programming Session:\n")
	fmt.Printf("  Session ID:        %s\n", sess.ID)
	fmt.Printf("  Multiplexer:       %s\n", m.Name())
	fmt.Printf("  Pane ID:           %s\n", sess.PaneHandle.PaneID)
	fmt.Printf("  Leader:            %s (callsign: %s)\n", sess.LeaderAgent, sess.LeaderCallsign)
	fmt.Printf("  Follower:          %s (callsign: %s, model: %s)\n", sess.FollowerAgent, sess.FollowerCallsign, sess.FollowerModel)
	fmt.Printf("  Working Directory: %s\n", sess.Cwd)
	fmt.Printf("  Read-Only:         %v\n", sess.ReadOnly)
	fmt.Printf("  Uptime:            %s\n", uptime)

	return nil
}

func runModels(args []string) error {
	agentName := "opencode"
	if len(args) > 0 {
		agentName = args[0]
	}

	ad, err := agent.Get(agentName)
	if err != nil {
		return err
	}

	models, err := ad.ListModels()
	if err != nil {
		return fmt.Errorf("failed to list models for %s: %w", agentName, err)
	}

	fmt.Printf("Models available for %s:\n", agentName)
	for _, m := range models {
		fmt.Printf("  - %s\n", m)
	}
	return nil
}
