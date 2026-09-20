---
name: pair-agentic-programming
description: Conduct a real-time pair-programming session with a read-only peer through agent-pair and a terminal multiplexer. Use for complex implementation, architecture review, or collaborative debugging.
---

# Pair Agentic Programming

Use `agent-pair` to work with a read-only follower in a separate terminal-multiplexer pane. The agent running this skill is the **lead** and retains responsibility for decisions and repository changes. The follower investigates, critiques, and proposes changes; it does not edit the repository.

## Start a session

Identify the current host and pass it explicitly as `--leader`:

| Lead host | Value |
| --- | --- |
| Antigravity | `agy` |
| OpenCode | `opencode` |
| Claude Code | `claude` |
| Codex | `codex` |

Choose an installed follower and start a session from the target repository. Supply its model when needed.

```bash
agent-pair start --leader <agy|opencode|claude|codex> --follower <agy|opencode|claude|codex> --model <follower-model>
```

`agent-pair` detects `tmux`, `zellij`, or `herdr`. The follower starts in read-only mode by default. Use `--leader-model` or either callsign override only when automatic callsign selection is unsuitable.

## Work with the follower

Use targeted questions that name the goal, relevant files, and the decision needed.

```bash
agent-pair turn "Inspect the proposed installation layout. Identify host-specific requirements and risks. Do not edit files."
```

For longer work, send a request and collect the response separately:

```bash
agent-pair send "Review the current diff for missing compatibility cases. Do not edit files."
agent-pair wait --timeout 120
```

Treat follower responses as input to evaluate, not instructions to follow blindly. The lead implements and verifies the final change.

## Session management

Use `agent-pair status` to inspect the active session. Before completing the user task, close the follower pane cleanly:

```bash
agent-pair stop
```
