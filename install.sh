#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="$PREFIX/bin"
OPENCODESKILLSDIR="${HOME}/.config/opencode/skills"
CLAUDESKILLSDIR="${HOME}/.claude/skills"
CODEXSKILLSDIR="${CODEX_HOME:-${HOME}/.codex}/skills"
AGYIDESKILLSDIR="${HOME}/.gemini/config/skills"
REPO="${AGENT_PAIR_REPO:-cpro/agent-pair}"
VERSION="${AGENT_PAIR_VERSION:-latest}"

mkdir -p "$BINDIR"

installed_binary=false
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

# 1. Try downloading pre-built release artifact from GitHub Releases
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) ARCH="" ;;
esac

if [ -n "$ARCH" ] && command -v curl >/dev/null 2>&1; then
    ASSET_NAME="agent-pair-${OS}-${ARCH}.tar.gz"
    if [ "$VERSION" = "latest" ]; then
        DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ASSET_NAME}"
    else
        DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"
    fi

    echo "==> Attempting to download prebuilt binary: ${ASSET_NAME}..."
    if curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ASSET_NAME}" 2>/dev/null; then
        echo "==> Extracting release artifact..."
        tar -xzf "${TMP_DIR}/${ASSET_NAME}" -C "$TMP_DIR"
        install -m 755 "${TMP_DIR}/agent-pair-${OS}-${ARCH}/agent-pair" "$BINDIR/agent-pair"
        installed_binary=true
        echo "==> Successfully installed prebuilt binary to $BINDIR/agent-pair"
    fi
fi

# 2. Fallback: Build from source if go is available
if [ "$installed_binary" = false ]; then
    if command -v go >/dev/null 2>&1 && [ -f "$SCRIPT_DIR/go.mod" ]; then
        echo "==> Building agent-pair binary from local source..."
        cd "$SCRIPT_DIR"
        mkdir -p "$SCRIPT_DIR/bin"
        go build -ldflags="-s -w" -o "$SCRIPT_DIR/bin/agent-pair" ./cmd/agent-pair
        install -m 755 "$SCRIPT_DIR/bin/agent-pair" "$BINDIR/agent-pair"
        installed_binary=true
        echo "==> Successfully built and installed binary to $BINDIR/agent-pair"
    else
        echo "Error: Could not download prebuilt release asset and 'go' compiler is not installed." >&2
        exit 1
    fi
fi

# 3. Locate and copy skill files
SKILL_SRC=""
if [ -d "$SCRIPT_DIR/skills/pair-agentic-programming" ]; then
    SKILL_SRC="$SCRIPT_DIR/skills/pair-agentic-programming"
elif [ -n "$ARCH" ] && [ -d "${TMP_DIR}/agent-pair-${OS}-${ARCH}/skills/pair-agentic-programming" ]; then
    SKILL_SRC="${TMP_DIR}/agent-pair-${OS}-${ARCH}/skills/pair-agentic-programming"
fi

if [ -n "$SKILL_SRC" ] && [ -d "$SKILL_SRC" ]; then
    install_skill() {
        skill_dir="$1"
        target="${skill_dir}/pair-agentic-programming"
        echo "==> Installing skill to $target..."
        mkdir -p "$skill_dir"
        rm -rf "$target"
        cp -R "$SKILL_SRC" "$target"
    }

    install_skill "$OPENCODESKILLSDIR"
    install_skill "$CLAUDESKILLSDIR"
    install_skill "$CODEXSKILLSDIR"
    install_skill "$AGYIDESKILLSDIR"
else
    echo "Warning: Could not find the complete skill bundle to install." >&2
fi

echo ""
echo "Successfully installed agent-pair!"
echo "  • CLI Binary: $BINDIR/agent-pair"
echo "  • OpenCode: $OPENCODESKILLSDIR/pair-agentic-programming"
echo "  • Claude Code: $CLAUDESKILLSDIR/pair-agentic-programming"
echo "  • Codex: $CODEXSKILLSDIR/pair-agentic-programming"
echo "  • Antigravity: $AGYIDESKILLSDIR/pair-agentic-programming"
echo ""
echo "Try running: agent-pair --help"
