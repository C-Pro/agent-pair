# Agent-Pair: Multi-Agent Pair Programming CLI & Skill

[![Release](https://github.com/C-Pro/agent-pair/actions/workflows/release.yml/badge.svg)](https://github.com/C-Pro/agent-pair/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Agent-Pair** is an extensible tool and agent skill that enables real-time, inter-process pair programming between autonomous AI coding agents.

It uses terminal multiplexer panes (**tmux**, **zellij**, or **herdr**) as visual IPC channels to pair any supported **Lead Agent** (Antigravity / `agy`, OpenCode, Claude Code, or Codex) with a read-only follower.

![Agent-Pair Demo](assets/demo.gif)

---

## Key Features

- **Dynamic Model-Driven Callsigns**: Radio callsigns automatically derive from active model names (e.g. `muse-spark-1.3`, `gemini-3.8-flash`, `qwen3.6-35b`, `claude-3-7-sonnet`).
- **Multi-Multiplexer Support**: Native drivers for **tmux**, **zellij**, and **herdr** with automatic environment detection.
- **Multi-Agent Adapters**: Pluggable drivers for **OpenCode**, **Antigravity (`agy`)**, **Claude Code**, and **Codex**.
- **Radio CQ Communication Protocol**: Standardized transmission turns (`[ CQ <sender-model> -> <recipient-model> ]` ... `[ <sender-model> over ]`) with robust text cleaning and turn extraction.
- **Read-Only Peer Protection**: Follower agents launch with strict read-only permissions (`--agent plan`, sandboxed bash, or tool restrictions) in the workspace directory, ensuring only the lead agent makes code modifications.
- **Prebuilt Binary Releases**: GitHub Actions builds standalone binaries for Linux, macOS, and Windows.
- **Universal Skill & Plugin Standards**: Compatible with Antigravity (`plugin.json`), OpenCode (`.agents/skills`), Claude Code (`.claude/skills`), and Codex plugin marketplaces.

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

### Method 1: Automated Script (Downloads Prebuilt Binary)
The `install.sh` script downloads the matching precompiled binary from GitHub Releases (falling back to building from source if needed):
```bash
curl -fsSL https://raw.githubusercontent.com/cpro/agent-pair/main/install.sh | bash
```
Or from a cloned repository:
```bash
./install.sh
```

### Method 2: Build from Source
```bash
make install
```

### Method 3: Antigravity Plugin
The repository root is an Antigravity plugin:
```bash
agy plugin install https://github.com/C-Pro/agent-pair
```

The script and `make install` install the same skill for all four hosts. They use the native global skill locations for OpenCode, Claude Code, Codex, and Antigravity. Restart the host after installation if it does not reload skills automatically.

---

## Quick Start

### 1. View Available Models
```bash
agent-pair models opencode
```

### 2. Launch a Pair Programming Session
Split the terminal window and initialize OpenCode with Muse in read-only mode (callsign automatically becomes `muse-spark-1.3`):
```bash
agent-pair start --leader codex --follower opencode --model opencode/muse-spark-1.3-contributor-free
```

### 3. Conduct a Collaborative Turn
Ask the peer agent for critique, alternative designs, or edge-case reviews:
```bash
agent-pair turn "How should we handle multiplexer timeouts gracefully in Go?"
```

Output:
```text
==> Turn sent to muse-spark-1.3 [%17]
[ CQ muse-spark-1.3 -> gemini ]

To handle multiplexer timeouts gracefully in Go:
1. Use context.WithTimeout on pane output polling loops.
2. Distinguish between agent thinking delay and dead multiplexer sockets.
3. Fallback to reading the latest snapshot from capture-pane before aborting.

[ muse-spark-1.3 over ]
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

## Skill Compatibility

`skills/pair-agentic-programming` is the canonical skill bundle. Its YAML frontmatter and instructions are provider-neutral; the running host is always the lead. Pass the matching value to `--leader` when starting a session:

| Lead host | `--leader` value | Installed skill location |
| --- | --- | --- |
| Antigravity | `agy` | `~/.gemini/config/skills/` |
| OpenCode | `opencode` | `~/.config/opencode/skills/` |
| Claude Code | `claude` | `~/.claude/skills/` |
| Codex | `codex` | `${CODEX_HOME:-~/.codex}/skills/` |

The root [plugin.json](plugin.json) packages the skill for Antigravity. [.codex-plugin/plugin.json](.codex-plugin/plugin.json) packages the same skill for Codex plugin marketplaces.

If `--leader auto` is used, `agent-pair` honors `AGENT_PAIR_LEADER` and detects Codex when `CODEX_HOME` is set. Pass `--leader` explicitly when detection is unavailable.

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

### Flags for `agent-pair start`
- `--mux`: Multiplexer (`auto`, `tmux`, `zellij`, `herdr`)
- `--follower`: Peer agent (`opencode`, `agy`, `claude`, `codex`)
- `--leader`: Leading agent (`agy`, `opencode`, `claude`, `codex`)
- `--model`: Model ID to pass to the follower agent (e.g. `opencode/muse-spark-1.3-contributor-free`)
- `--leader-model`: Model ID of the leader agent (e.g. `gemini-3.8-flash`)
- `--follower-callsign`: Override follower callsign (defaults to model name)
- `--leader-callsign`: Override leader callsign (defaults to model name)
- `--read-only`: Enforce read-only mode on follower (default: `true`)
- `--cwd`: Working directory (defaults to current directory)
- `--direction`: Split direction (`horizontal`, `vertical`)
- `--size`: Split size percentage (default: `45%`)

---

## License

MIT License. See [LICENSE](LICENSE) for details.
