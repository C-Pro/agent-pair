# Agent-Pair: Multi-Agent Pair Programming CLI & Skill

[![Go](https://img.shields.io/badge/go-1.22%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Agent-Pair** is an extensible tool and agent skill that enables real-time, inter-process pair programming between autonomous AI coding agents.

It uses terminal multiplexer panes (**tmux**, **zellij**, or **herdr**) as visual IPC channels to pair a **Lead Agent** (e.g. Antigravity / `gemini`) with a **Follower Agent** (e.g. OpenCode / `muse`, Claude Code, or Codex).

---

## Key Features

- **Multi-Multiplexer Support**: Native drivers for **tmux**, **zellij**, and **herdr** with automatic environment detection.
- **Multi-Agent Adapters**: Pluggable drivers for **OpenCode**, **Antigravity (`agy`)**, **Claude Code**, and **Codex**.
- **Radio CQ Communication Protocol**: Standardized transmission turns (`[ CQ <sender> -> <recipient> ]` ... `[ <sender> over ]`) with robust text cleaning and turn extraction.
- **Read-Only Peer Protection**: Follower agents launch with strict read-only permissions (`--agent plan`, sandboxed bash, or tool restrictions) in the workspace directory, ensuring only the lead agent makes code modifications.
- **Zero External Dependencies**: Pure Go standard library implementation for lightweight compilation, fast startup, and universal portability.
- **Distributable Skill**: Includes a standard `SKILL.md` ready for installation into `~/.agents/skills/` or AI skill marketplaces.

---

## Architecture

```
                        ┌────────────────────────────────────────────────────────┐
                        │                 agent-pair CLI (Go)                    │
                        └──────────┬───────────────────────────────┬─────────────┘
                                   │                               │
                ┌──────────────────▼───────────────┐ ┌─────────────▼──────────────────┐
                │       Multiplexer Driver         │ │         Agent Adapters         │
                │  (Auto-detects or configured)    │ │  (Leader & Follower roles)     │
                ├──────────────────────────────────┤ ├────────────────────────────────┤
                │ • tmux   (split/buffer/capture)  │ │ • agy      (gemini callsign)   │
                │ • zellij (run/write-chars/dump)  │ │ • opencode (muse/zen callsign) │
                │ • herdr  (pane API / socket)     │ │ • claude   (drafted)           │
                │                                  │ │ • codex    (drafted)           │
                └──────────────────────────────────┘ └────────────────────────────────┘
```

---

## Installation

### Using the installer script
```bash
./install.sh
```

### Or using `make`
```bash
make install
```

This installs:
- Binary to `~/.local/bin/agent-pair`
- Skill to `~/.agents/skills/pair-agentic-programming` (and `~/.gemini/config/skills/`)

---

## Quick Start

### 1. View Available Models
```bash
agent-pair models opencode
```

### 2. Launch a Pair Programming Session
Split the terminal window and initialize OpenCode with Muse in read-only mode:
```bash
agent-pair start --follower opencode --model opencode/muse-spark-1.3-contributor-free
```

### 3. Conduct a Collaborative Turn
Ask the peer agent for critique, alternative designs, or edge-case reviews:
```bash
agent-pair turn "How should we handle multiplexer timeouts gracefully in Go?"
```

### 4. Check Status
```bash
agent-pair status
```

### 5. Terminate and Clean Up
```bash
agent-pair stop
```

---

## CLI Reference

| Subcommand | Description |
|---|---|
| `start` | Launches follower in a demuxer pane and runs the protocol handshake |
| `send` | Transmits a formatted turn message to the follower |
| `wait` | Waits for follower generation to complete and extracts response |
| `turn` | Combined `send` + `wait` shortcut |
| `status` | Shows details of the currently active session |
| `stop` | Gracefully terminates follower and closes the multiplexer pane |
| `models [agent]` | Lists supported models for the chosen agent |

### Key Flags for `agent-pair start`
- `--mux`: Multiplexer (`auto`, `tmux`, `zellij`, `herdr`)
- `--follower`: Peer agent (`opencode`, `agy`, `claude`, `codex`)
- `--leader`: Leading agent (`agy`, `opencode`, `claude`, `codex`)
- `--model`: Model ID to pass to the follower agent
- `--follower-callsign`: Radio callsign for follower (defaults based on model)
- `--leader-callsign`: Radio callsign for leader (defaults based on agent)
- `--read-only`: Enforce read-only mode on follower (default: `true`)
- `--cwd`: Working directory (defaults to current directory)
- `--direction`: Split direction (`horizontal`, `vertical`)
- `--size`: Split size percentage (default: `45%`)

---

## License

MIT License. See [LICENSE](LICENSE) for details.
