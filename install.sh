#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="$PREFIX/bin"
SKILLSDIR="${HOME}/.agents/skills"
GEMINIDIR="${HOME}/.gemini/config/skills"
REPO="${AGENT_PAIR_REPO:-cpro/agent-pair}"
VERSION="${AGENT_PAIR_VERSION:-latest}"

mkdir -p "$BINDIR"

installed_binary=false

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
    TMP_DIR="$(mktemp -d)"
    if curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ASSET_NAME}" 2>/dev/null; then
        echo "==> Extracting release artifact..."
        tar -xzf "${TMP_DIR}/${ASSET_NAME}" -C "$TMP_DIR"
        install -m 755 "${TMP_DIR}/agent-pair-${OS}-${ARCH}/agent-pair" "$BINDIR/agent-pair"
        installed_binary=true
        echo "==> Successfully installed prebuilt binary to $BINDIR/agent-pair"
    fi
    rm -rf "$TMP_DIR"
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

# 3. Register skill
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
