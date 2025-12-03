# -------------------------
# Dead simple mkenv Makefile
# -------------------------

# Host target (override if needed: make host TARGET_OS=linux TARGET_ARCH=amd64)
TARGET_OS   ?= darwin
TARGET_ARCH ?= arm64

# Output dirs
HOST_OUT   := dist/$(TARGET_OS)-$(TARGET_ARCH)
AGENT_OUT  := internal/agentdist/bin

# Default target: build everything
all: fmt agent host

fmt:
	go fmt ./...

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
# Build ONE host binary (cmd/mkenv)
# -------------------------
host:
	mkdir -p $(HOST_OUT)
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) CGO_ENABLED=0 \
		go build -o $(HOST_OUT)/mkenv ./cmd/mkenv

