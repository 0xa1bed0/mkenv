# Host target (override if needed: make host TARGET_OS=linux TARGET_ARCH=amd64)
TARGET_OS   ?= darwin
TARGET_ARCH ?= arm64

# Output dirs
HOST_OUT   := dist/$(TARGET_OS)-$(TARGET_ARCH)
AGENT_OUT  := internal/agentdist/bin

# Git commit hash (empty if not in a git repo or zip download)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)

# Version for user builds: compiled-<commit> if git available, otherwise compiled
USER_VERSION := $(if $(GIT_COMMIT),compiled-$(GIT_COMMIT),compiled)

# Default target: build for users
all: fmt agent host-user

# Development build (for mkenv developers)
dev: fmt agent host-dev

host-user:
	mkdir -p $(HOST_OUT)
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) CGO_ENABLED=0 \
		go build -ldflags "-X github.com/0xa1bed0/mkenv/internal/version.Version=$(USER_VERSION)" -o $(HOST_OUT)/mkenv ./cmd/mkenv

host-dev:
	mkdir -p $(HOST_OUT)
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) CGO_ENABLED=0 \
		go build -ldflags "-X github.com/0xa1bed0/mkenv/internal/version.Version=local" -o $(HOST_OUT)/mkenv ./cmd/mkenv

fmt:
	go fmt ./...

test:
	go test ./...

# -------------------------
# Build Linux agent binaries for embedding
# -------------------------
agent:
	mkdir -p $(AGENT_OUT)/linux_amd64
	mkdir -p $(AGENT_OUT)/linux_arm64

	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build -o /tmp/mkenv-agent-amd64 ./cmd/mkenv-agent
	gzip -9 -c /tmp/mkenv-agent-amd64 > $(AGENT_OUT)/linux_amd64/mkenv-agent.gz

	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
		go build -o /tmp/mkenv-agent-arm64 ./cmd/mkenv-agent
	gzip -9 -c /tmp/mkenv-agent-arm64 > $(AGENT_OUT)/linux_arm64/mkenv-agent.gz

	rm -f /tmp/mkenv-agent-amd64 /tmp/mkenv-agent-arm64

# -------------------------
# Build host binary with explicit version (used by CI)
# -------------------------
host:
ifndef VERSION
	$(error VERSION is required for 'make host'. Use 'make' for user builds or 'make dev' for development)
endif
	mkdir -p $(HOST_OUT)
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) CGO_ENABLED=0 \
		go build -ldflags "-X github.com/0xa1bed0/mkenv/internal/version.Version=$(VERSION)" -o $(HOST_OUT)/mkenv ./cmd/mkenv

