---
name: pair-agentic-programming
description: Multi-agent pair programming via terminal multiplexer IPC (tmux, zellij, herdr) with a peer agent (OpenCode, Claude, Codex). Use when complex architecture, deep code review, or collaborative brainstorming benefits from a dual-agent perspective.
---

# Pair Agentic Programming Skill

This skill enables Antigravity (`agy`) to conduct a real-time, multi-agent pair programming session with a peer AI agent (such as **OpenCode** running `muse` or local models, **Claude Code**, or **Codex**) through a dedicated terminal multiplexer pane (`tmux`, `zellij`, or `herdr`).

---

## 1. Roles and Responsibilities

- **Lead Agent (`agy` / callsign derived from model, e.g. `gemini` or `gemini-3.8-flash`)**:
  - Directs the programming task, asks questions, requests reviews, and drives execution.
  - Retains write permissions and makes actual code changes in the repository.
- **Follower Agent (e.g., `opencode` / callsign derived from model, e.g. `muse-spark-1.3`, `big-pickle`, `qwen3.6-35b`)**:
  - Operates in **read-only mode** (`--agent plan`) in the same working directory.
  - Analyzes files, critiques architecture, proposes alternative implementations, and checks edge cases.

---

## 2. Communication Protocol (Radio CQ)

Messages exchanged between agents follow a radio-style transmission format where callsigns dynamically follow the model names:

- **Turn Header**:
  ```text
  [ CQ <sender-model> -> <recipient-model> ]
  ```
  *(e.g., `[ CQ gemini -> muse-spark-1.3 ]` or `[ CQ muse-spark-1.3 -> gemini ]`)*

- **Turn Footer**:
  ```text
  [ <sender-model> over ]
  ```

- **Session Sign-off**:
  ```text
  [ <sender-model> out ]
  ```

The companion CLI tool `agent-pair` automatically extracts clean callsigns from model strings and injects these markers. Explicit callsign overrides can still be provided via `--follower-callsign` and `--leader-callsign`.

---

## 3. Workflow Procedure

### Step 1: Discover Available Models
Check the available models for your preferred follower agent:
```bash
agent-pair models opencode
```

### Step 2: Initialize the Pair Session
Launch the follower agent in a new split pane inside the active multiplexer (auto-detects `tmux`, `zellij`, or `herdr`):
```bash
# Example: Pair with OpenCode's free Muse model
agent-pair start --follower opencode --model opencode/muse-spark-1.3-contributor-free

# Or specify a custom multiplexer and callsigns:
agent-pair start --mux tmux --follower opencode --model opencode/muse-spark-1.3-contributor-free --follower-callsign muse --leader-callsign gemini
```

The tool will:
1. Split the screen (side-by-side).
2. Boot the follower agent with read-only permissions in the current workspace directory.
3. Transmit the protocol bootstrap prompt.
4. Verify the follower's acknowledgment (`ACK`).

### Step 3: Conduct Pair Programming Turns
Send questions, propose designs, or request code reviews using the `turn` subcommand:
```bash
agent-pair turn "We need to refactor the error handling in pkg/mux/tmux.go. What are the edge cases when tmux capture-pane returns empty output?"
```

You can also separate send and wait if performing intermediate tasks:
```bash
# Send turn
agent-pair send "Please inspect the git diff and look for memory leaks."

# Wait for follower's response
agent-pair wait --timeout 90
```

### Step 4: Check Session Status
Inspect the live session state at any time:
```bash
agent-pair status
```

### Step 5: Graceful Teardown
When the collaboration is complete, cleanly exit the peer agent and close the demuxer pane:
```bash
agent-pair stop
```

---

## 4. Best Practices for the Lead Agent

1. **Be Specific in Turns**: Frame targeted questions or share specific file paths and line numbers so the peer agent can inspect them directly.
2. **Synthesize Peer Feedback**: Evaluate the peer's critique critically. Adopt valid suggestions and challenge questionable ones in subsequent turns.
3. **Always Clean Up**: Ensure `agent-pair stop` is called before concluding the user task to prevent orphaned multiplexer panes.
