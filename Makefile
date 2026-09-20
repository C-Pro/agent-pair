PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
OPENCODESKILLSDIR ?= $(HOME)/.config/opencode/skills
CLAUDESKILLSDIR ?= $(HOME)/.claude/skills
CODEXSKILLSDIR ?= $(or $(CODEX_HOME),$(HOME)/.codex)/skills
AGYIDESKILLSDIR ?= $(HOME)/.gemini/config/skills

.PHONY: all build test clean install

all: build

build:
	mkdir -p bin
	go build -ldflags="-s -w" -o bin/agent-pair ./cmd/agent-pair

test:
	go test -v ./...

install: build
	mkdir -p $(BINDIR)
	install -m 755 bin/agent-pair $(BINDIR)/agent-pair
	@for skill_dir in $(OPENCODESKILLSDIR) $(CLAUDESKILLSDIR) $(CODEXSKILLSDIR) $(AGYIDESKILLSDIR); do \
		mkdir -p "$$skill_dir"; \
		rm -rf "$$skill_dir/pair-agentic-programming"; \
		cp -R $(CURDIR)/skills/pair-agentic-programming "$$skill_dir/pair-agentic-programming"; \
		done
	@echo "==> Installed agent-pair to $(BINDIR)/agent-pair"
	@echo "==> Installed pair-agentic-programming for agy, OpenCode, Claude Code, and Codex"

clean:
	rm -rf bin
