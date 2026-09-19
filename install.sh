#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="$PREFIX/bin"
SKILLSDIR="${HOME}/.agents/skills"
GEMINIDIR="${HOME}/.gemini/config/skills"

echo "==> Building agent-pair binary..."
cd "$SCRIPT_DIR"
mkdir -p "$SCRIPT_DIR/bin"
go build -ldflags="-s -w" -o "$SCRIPT_DIR/bin/agent-pair" ./cmd/agent-pair

echo "==> Installing binary to $BINDIR/agent-pair..."
mkdir -p "$BINDIR"
install -m 755 "$SCRIPT_DIR/bin/agent-pair" "$BINDIR/agent-pair"

echo "==> Registering skill in $SKILLSDIR..."
mkdir -p "$SKILLSDIR"
ln -sfn "$SCRIPT_DIR/skills/pair-agentic-programming" "$SKILLSDIR/pair-agentic-programming"

if [ -d "$HOME/.gemini/config" ]; then
    mkdir -p "$GEMINIDIR"
    ln -sfn "$SCRIPT_DIR/skills/pair-agentic-programming" "$GEMINIDIR/pair-agentic-programming"
fi

echo ""
echo "Successfully installed agent-pair!"
echo "  • CLI Binary: $BINDIR/agent-pair"
echo "  • Agent Skill: $SKILLSDIR/pair-agentic-programming"
echo ""
echo "Try running: agent-pair --help"
