PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
SKILLSDIR ?= $(HOME)/.agents/skills
GEMINIDIR ?= $(HOME)/.gemini/config/skills

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
	mkdir -p $(SKILLSDIR)
	rm -rf $(SKILLSDIR)/pair-agentic-programming
	cp -R $(CURDIR)/skills/pair-agentic-programming $(SKILLSDIR)/pair-agentic-programming
	@if [ -d "$(HOME)/.gemini/config" ]; then \
		mkdir -p $(GEMINIDIR); \
		rm -rf $(GEMINIDIR)/pair-agentic-programming; \
		cp -R $(CURDIR)/skills/pair-agentic-programming $(GEMINIDIR)/pair-agentic-programming; \
	fi
	@echo "==> Installed agent-pair to $(BINDIR)/agent-pair"
	@echo "==> Copied skill to $(SKILLSDIR)/pair-agentic-programming"

clean:
	rm -rf bin
