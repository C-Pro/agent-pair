package main

import (
	"errors"
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
  claude, codex, agy, opencode

NOTE:
  The follower reads this repository and everything you send it, under its own
  provider account. Choose a follower model your organization has approved to
  receive this code.

EXAMPLES:
  # Pair Claude Code with a read-only Claude follower on Opus 5
  agent-pair start --leader claude --follower claude --model claude-opus-5

  # Send a turn and wait for reply
  agent-pair turn "What are the tradeoffs of using Go channels vs mutexes here?"

  # Stop session and close pane
  agent-pair stop`)
}

func runStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ExitOnError)

	muxName := fs.String("mux", "auto", "Multiplexer to use (auto, tmux, zellij, herdr)")
	followerName := fs.String("follower", "claude", "Follower agent (claude, codex, agy, opencode)")
	leaderName := fs.String("leader", "auto", "Leader agent (auto, agy, opencode, claude, codex)")
	model := fs.String("model", "", "Model identifier for follower")
	effort := fs.String("effort", "", "Follower reasoning effort (low, medium, high)")
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

	existing, err := loadActiveSession()
	if err != nil && !errors.Is(err, session.ErrNoActiveSession) {
		return err
	}
	if existing != nil && !*force {
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
	leaderAdapter, err := resolveLeader(*leaderName)
	if err != nil {
		return err
	}

	// Resolve model. There is deliberately no default: the follower reads this
	// repository, so which provider and tier receives it is the operator's call.
	targetModel := *model
	targetEffort := *effort
	if targetEffort == "" {
		targetEffort = os.Getenv("AGENT_PAIR_EFFORT")
	}

	// Resolve callsigns: always follow model names unless explicitly overridden
	fCallsign := *followerCallsign
	if fCallsign == "" {
		fCallsign = agent.DeriveCallsign(targetModel, followerAdapter.Name())
	}

	lCallsign := *leaderCallsign
	if lCallsign == "" {
		lModel := *leaderModelFlag
		if lModel == "" {
			lModel = leaderModelFromEnv(leaderAdapter.Name())
		}
		lCallsign = agent.DeriveCallsign(lModel, leaderAdapter.DefaultCallsign(""))
	}

	// Build follower launch command
	cmd, env, err := followerAdapter.BuildLaunchCommand(agent.LaunchOptions{
		Model:    targetModel,
		Effort:   targetEffort,
		ReadOnly: *readOnly,
		Cwd:      targetCwd,
		Callsign: fCallsign,
	})
	if err != nil {
		return fmt.Errorf("failed to build launch command: %w", err)
	}
	cmd = agent.SanitizeLaunchCommand(cmd, env)

	title := fmt.Sprintf("[pair:%s]", fCallsign)
	fmt.Printf("==> Launching %s (%s) in %s pane...\n", followerAdapter.Name(), fCallsign, m.Name())
	if existing != nil && *force {
		oldMux, err := mux.Get(existing.MuxName)
		if err != nil {
			return fmt.Errorf("failed to resolve existing session multiplexer: %w", err)
		}
		if err := oldMux.ClosePane(existing.PaneHandle); err != nil {
			return fmt.Errorf("failed to close existing follower pane: %w", err)
		}
		if err := session.Clear(); err != nil {
			return fmt.Errorf("failed to clear existing session: %w", err)
		}
	}

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
	startupComplete := false
	defer func() {
		if !startupComplete {
			_ = m.ClosePane(handle)
			_ = session.Clear()
		}
	}()

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
		turnID := protocol.NewTurnID()
		sess.TurnID = turnID
		if err := session.Save(sess); err != nil {
			return fmt.Errorf("failed to save bootstrap turn id: %w", err)
		}
		// The handshake travels in the same envelope as every other turn, so
		// its acknowledgement is matched the same way.
		bootstrap := protocol.FormatTurn(lCallsign, fCallsign,
			protocol.FormatBootstrapPrompt(lCallsign, fCallsign, targetCwd, turnID, *readOnly), turnID)
		if err := m.SendText(handle, bootstrap); err != nil {
			return fmt.Errorf("failed to send bootstrap prompt: %w", err)
		}

		fmt.Printf("==> Awaiting protocol acknowledgment...\n")
		ackDone := false
		ackBaseline := 0
		ackStart := time.Now()
		for time.Since(ackStart) < timeout {
			time.Sleep(1 * time.Second)
			output, err := m.CaptureOutput(handle)
			if err != nil {
				continue
			}
			if protocol.IsTurnComplete(output, fCallsign, lCallsign, turnID) && followerAdapter.IsTurnFinished(output, fCallsign, "") {
				ackDone = true
				ackBaseline = protocol.CountTurnEndMarkers(output, fCallsign, "")
				if body, found := protocol.ExtractLatestTurn(output, fCallsign, lCallsign, turnID); found {
					fmt.Printf("\n[ %s ACK RECEIVED ]\n%s\n\n", fCallsign, body)
				}
				break
			}
		}

		if !ackDone {
			fmt.Printf("Warning: ACK marker not detected yet, but session is initialized.\n")
		} else {
			sess.ResponseBaseline = ackBaseline
			if err := session.Save(sess); err != nil {
				return fmt.Errorf("failed to save bootstrap response watermark: %w", err)
			}
		}
	}

	fmt.Printf("==> Pair programming session established!\n")
	fmt.Printf("    Leader:   %s (%s)\n", leaderAdapter.Name(), lCallsign)
	fmt.Printf("    Follower: %s (%s, model: %s)\n", followerAdapter.Name(), fCallsign, targetModel)
	fmt.Printf("    Channel:  %s (pane: %s)\n", m.Name(), handle.PaneID)
	startupComplete = true
	return nil
}

func resolveLeader(name string) (agent.AgentAdapter, error) {
	if name != "auto" {
		return agent.Get(name)
	}

	if configured := os.Getenv("AGENT_PAIR_LEADER"); configured != "" {
		return agent.Get(configured)
	}

	type indicator struct {
		agent string
		envs  []string
	}
	indicators := []indicator{
		{agent: "codex", envs: []string{"CODEX_SESSION_ID", "CODEX_THREAD_ID"}},
		{agent: "claude", envs: []string{"CLAUDECODE"}},
		{agent: "opencode", envs: []string{"OPENCODE"}},
		{agent: "agy", envs: []string{"ANTIGRAVITY_AGENT", "ANTIGRAVITY_CONVERSATION_ID", "AGY_SESSION_ID", "ANTIGRAVITY_SESSION_ID"}},
	}

	detected := ""
	for _, candidate := range indicators {
		for _, envName := range candidate.envs {
			if os.Getenv(envName) == "" {
				continue
			}
			if detected != "" && detected != candidate.agent {
				return nil, fmt.Errorf("ambiguous leader environment (%s and %s); pass --leader explicitly", detected, candidate.agent)
			}
			detected = candidate.agent
			break
		}
	}
	if detected != "" {
		return agent.Get(detected)
	}

	return nil, fmt.Errorf("could not detect the leader; pass --leader agy, opencode, claude, or codex")
}

func leaderModelFromEnv(leader string) string {
	if model := os.Getenv("AGENT_PAIR_LEADER_MODEL"); model != "" {
		return model
	}

	if leader == "agy" {
		return os.Getenv("GEMINI_MODEL")
	}
	return ""
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

func loadActiveSession() (*session.Session, error) {
	sess, err := session.Load()
	if err != nil {
		return nil, err
	}
	m, err := mux.Get(sess.MuxName)
	if err != nil {
		return nil, err
	}
	alive, err := m.PaneAlive(sess.PaneHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to check follower pane: %w", err)
	}
	if !alive {
		if err := session.Clear(); err != nil {
			return nil, fmt.Errorf("failed to clear stale session: %w", err)
		}
		return nil, session.ErrNoActiveSession
	}
	return sess, nil
}

func runSend(args []string) error {
	msg, err := readMessageFromArgsOrStdin(args)
	if err != nil {
		return err
	}

	sess, err := loadActiveSession()
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
	// Each turn carries its own id, so a marker in scrollback -- or one inside
	// an earlier reply quoted back into this prompt -- cannot be read as this
	// turn completing. The watermark stays as a fallback for followers that
	// drop the id.
	sess.TurnID = protocol.NewTurnID()
	sess.ResponseBaseline = protocol.CountTurnEndMarkers(output, sess.FollowerCallsign, "")
	if err := session.Save(sess); err != nil {
		return fmt.Errorf("failed to save response watermark: %w", err)
	}

	formatted := protocol.FormatTurn(sess.LeaderCallsign, sess.FollowerCallsign, msg, sess.TurnID)
	if err := m.SendText(sess.PaneHandle, formatted); err != nil {
		return fmt.Errorf("failed to send turn message: %w", err)
	}

	fmt.Printf("==> Turn sent to %s [#%s]\n", sess.FollowerCallsign, sess.TurnID)
	return nil
}

func runWait(args []string) error {
	fs := flag.NewFlagSet("wait", flag.ExitOnError)
	timeoutSec := fs.Int("timeout", 120, "Timeout in seconds to wait for peer response")
	raw := fs.Bool("raw", false, "Print raw uncleaned screen output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	sess, err := loadActiveSession()
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

		// With a turn id the reply is matched by id, or by an untagged marker
		// that appears after this turn's prompt echo. Without one -- `wait`
		// called with no preceding `send` -- fall back to the scrollback
		// watermark taken when the session was last written.
		complete := protocol.CountTurnEndMarkers(output, sess.FollowerCallsign, "") > sess.ResponseBaseline
		if sess.TurnID != "" {
			complete = protocol.IsTurnComplete(output, sess.FollowerCallsign, sess.LeaderCallsign, sess.TurnID)
		}

		if complete && followerAdapter.IsTurnFinished(output, sess.FollowerCallsign, "") {
			if *raw {
				fmt.Println(output)
				return nil
			}

			// Printing a reply whose beginning never reached us, under a
			// header we synthesize, would pass a fragment off as the answer.
			if !protocol.ReplyStartVisible(output, sess.FollowerCallsign, sess.LeaderCallsign, sess.TurnID) {
				return fmt.Errorf("the reply from %s to #%s is longer than the captured follower output, so its beginning is missing; use 'agent-pair turn' to ask %s to resend it in shorter parts",
					sess.FollowerCallsign, sess.TurnID, sess.FollowerCallsign)
			}

			if body, found := protocol.ExtractLatestTurn(output, sess.FollowerCallsign, sess.LeaderCallsign, sess.TurnID); found {
				fmt.Printf("[ CQ %s -> %s #%s ]\n\n%s\n\n[ %s over #%s ]\n",
					sess.FollowerCallsign, sess.LeaderCallsign, sess.TurnID, body, sess.FollowerCallsign, sess.TurnID)
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
	sess, err := loadActiveSession()
	if err != nil {
		return err
	}

	m, err := mux.Get(sess.MuxName)
	if err != nil {
		return err
	}

	var cleanupErrs []error
	followerAdapter, err := agent.Get(sess.FollowerAgent)
	if err != nil {
		// Fallback to exit
		if sendErr := m.SendText(sess.PaneHandle, "exit"); sendErr != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("failed to ask follower to exit: %w", sendErr))
		}
	} else {
		if sendErr := m.SendText(sess.PaneHandle, followerAdapter.StopCommand()); sendErr != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("failed to ask follower to exit: %w", sendErr))
		}
	}

	time.Sleep(1 * time.Second)
	if closeErr := m.ClosePane(sess.PaneHandle); closeErr != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("failed to close follower pane: %w", closeErr))
	}
	if clearErr := session.Clear(); clearErr != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("failed to clear session state: %w", clearErr))
	}
	if len(cleanupErrs) > 0 {
		return errors.Join(cleanupErrs...)
	}

	fmt.Printf("==> Pair programming session closed cleanly.\n")
	return nil
}

func runStatus(args []string) error {
	sess, err := loadActiveSession()
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
	agentName := "claude"
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
