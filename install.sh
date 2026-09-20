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

if [ -z "$SKILL_SRC" ] && command -v curl >/dev/null 2>&1; then
    echo "==> Fetching skill definition..."
    REMOTE_SKILL_DIR="${TMP_DIR}/remote-skill/pair-agentic-programming"
    mkdir -p "$REMOTE_SKILL_DIR"
    if curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/skills/pair-agentic-programming/SKILL.md" -o "${REMOTE_SKILL_DIR}/SKILL.md" 2>/dev/null; then
        SKILL_SRC="$REMOTE_SKILL_DIR"
    fi
fi

if [ -n "$SKILL_SRC" ] && [ -d "$SKILL_SRC" ]; then
    echo "==> Installing skill to $SKILLSDIR..."
    mkdir -p "$SKILLSDIR"
    rm -rf "$SKILLSDIR/pair-agentic-programming"
    cp -R "$SKILL_SRC" "$SKILLSDIR/pair-agentic-programming"

    if [ -d "$HOME/.gemini/config" ]; then
        echo "==> Installing skill to $GEMINIDIR..."
        mkdir -p "$GEMINIDIR"
        rm -rf "$GEMINIDIR/pair-agentic-programming"
        cp -R "$SKILL_SRC" "$GEMINIDIR/pair-agentic-programming"
    fi
else
    echo "Warning: Could not find skill files to install." >&2
fi

echo ""
echo "Successfully installed agent-pair!"
echo "  • CLI Binary: $BINDIR/agent-pair"
echo "  • Agent Skill: $SKILLSDIR/pair-agentic-programming"
echo ""
echo "Try running: agent-pair --help"
