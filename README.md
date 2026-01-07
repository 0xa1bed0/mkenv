> [!WARNING]
>
> ## 🚨 Early Stage Warning
>
> mkenv is **experimental software**.
>
> You should expect:
>
> * breaking changes
> * incomplete features
> * instability
>
> We’re releasing it early so developers can try it and give feedback. Thanks for being part of shaping mkenv!

# mkenv

[![Website](https://img.shields.io/badge/website-online-green)](https://mkenv.sh)
[![License: Elastic License 2.0](https://img.shields.io/badge/license-Elastic%20License%202.0-blue)](#license)
[![Contributions Welcome](https://img.shields.io/badge/contributions-welcome-brightgreen)](../../issues)
![Status: Active](https://img.shields.io/badge/status-active-success)

**mkenv** creates isolated Docker development environments in seconds. It automatically detects your project's requirements and builds a complete dev container with the right tools, configs, and caches - without cluttering your host system.

Works with Neovim, Tmux, and other tools. Zero configuration required.

---

## ⚡ Quickstart

```sh
# Install via Homebrew
brew tap 0xa1bed0/mkenv
brew install mkenv

# Create isolated dev environment
cd /path/to/your/project
mkenv .
```

## 🚀 Key Features

### **Zero‑Config Environment Detection (the core feature)**

`mkenv` automatically analyzes your project and determines which development environment you need.

* Detects languages (Go, Node.js, Python, Rust, etc.)
* Infers toolchain versions from `go.mod`, `package.json`, `.nvmrc`, and similar files
* Generates a Docker environment without configuration files

This is the main differentiator from Devcontainers, Nix, or Compose setups.

### **Instant Dev Environments**

```sh
mkenv .
```

Creates a complete Docker dev environment with:

* Auto-generated Dockerfile
* Language runtimes and dev tools
* Your project mounted inside
* Cached dependencies and build artifacts
* Optional: editor configs, tmux, zsh

### **Disposable Containers, Persistent Caches**

Containers are temporary and removed between sessions, but mkenv persists what matters:

* Shell history
* Editor configs (tmux, neovim)
* Language build caches
* Dependency caches (npm, go, pip, etc.)

Result: **Fresh containers with warm caches**. You can reset or disable caching anytime.

### **Currently Supported**

**Languages:**
* Go
* Node.js

**AI Coding Tools:**
* Claude Code
* Codex

**Editors & Tools:**
* Neovim
* Tmux

**Infrastructure Tools:**
* Pulumi

---

## 📦 What Problems mkenv Solves

### **No more global clutter**

No global Node, Go, Python, Rust, Bun, Deno, pnpm, etc.

### **Fully isolated dev machines**

Every project runs in its own sandbox.

### **Reproducible and debuggable**

Your workspace is built deterministically:

* Auto-generated Dockerfile based on detected requirements
* Hash-based cache keys ensure repeatable builds
* Isolated build caches and volumes per project

### **Fast rebuilds**

Dependencies and build artifacts are cached, making rebuilds nearly instant.

---

## 🧠 How It Works

### **1. Detect Your Environment**

mkenv scans your project directory and detects:

* Languages (Go, Node.js, Python, Rust, etc.)
* Required packages and dependencies
* Toolchain versions

### **2. Build the Container**

mkenv generates and builds a Dockerfile with:

* Appropriate base image
* Language runtimes and tools
* System packages
* Security defaults (non-root user, isolated environment)

### **3. Run and Connect**

* Container starts with an interactive shell
* Your project is mounted inside
* All dependencies are cached for fast rebuilds
* Terminal resizes dynamically

---

## 🛠 Usage

### **Create / enter environment:**

```sh
mkenv .
```

### **Configuration with .mkenv files (optional)**

While mkenv works with zero configuration, you can customize behavior using `.mkenv` files. These files use the same options as command-line flags.

**Example - Install tools for all projects in a directory:**

Create `~/projects/.mkenv`:
```json
{
  "enabled_bricks": ["codex", "nvim", "tmux"]
}
```

Now `mkenv .` in any project under `~/projects/` automatically includes these tools.

**Example - Project-specific configuration:**

Create `.mkenv` in your project root:
```json
{
  "enabled_bricks": ["claude-code"],
  "volumes": ["~/datasets:/data"],
  "disable_auto": false
}
```

**Example - Install additional system packages:**

Create `.mkenv` in your project root:
```json
{
  "extra_pkgs": ["git", "curl", "vim", "htop", "jq"]
}
```

These packages will be installed via the system's package manager (e.g., apt-get on Debian-based systems).

**How it works:**
- mkenv searches from your project directory up to the root
- All `.mkenv` files found are loaded and merged (root → project)
- Child configs override parent configs
- Command-line flags override all `.mkenv` files

**Available fields:**
- `enabled_bricks` - Tools to install (e.g., `["codex", "claude-code", "nvim", "tmux", "pulumi"]`)
- `disabled_bricks` - Tools to exclude from auto-detection
- `extra_pkgs` - Additional system packages to install (e.g., `["git", "curl", "vim"]`)
- `volumes` - Additional directories to mount (e.g., `["~/data:/data"]`)
- `disable_auto` - Disable automatic language detection (default: `false`)

This lets you set organization-wide defaults, team preferences, or project-specific requirements without repeating flags.

### **Exit environment:**

Exit the shell with `exit` or `Ctrl+D`. The container is removed automatically.

**Note for tmux users:** If you detach from tmux (`Ctrl+b d`) and it's the last attached session, the container will exit and be removed.

---

## 📁 Volumes & Caching

### **Project volume**

Your project is mounted read/write.

### **Cache volumes**

Caches are namespaced per container:

```
cache_<container>_npm
cache_<container>_go_mod
cache_<container>_pip
```

mkenv automatically creates and reuses volumes for optimal performance.

---

## 🔐 Security Considerations

* System commands run as root only during image build
* Container processes run as non-root user
* Dependencies are installed inside the container, never on your host
* Cache files are validated before use
* No arbitrary container configuration overrides

---

## 🔐 Guardrails and Policy Engine

mkenv includes a comprehensive policy engine that enforces security guardrails at multiple levels. Policies can be configured globally or per-project using a `policy.json` file.

### Security Scanning

**Secret Detection:**
* Scans project files before creating containers
* Warns if secrets, SSH keys, or credentials are detected
* Requires explicit confirmation to proceed if sensitive files are found

**Restricted Directories:**
* Automatically blocks dangerous folders like `~/.ssh`, `~/.aws`, `~/.config` from being mounted
* These restrictions cannot be overridden to prevent accidental credential exposure

### Policy Configuration

Create a `policy.json` file in your mkenv config directory to enforce organizational policies:

```json
{
  "disabled_bricks": ["codex"],
  "enabled_bricks": ["nvim", "tmux"],
  "allowed_mount_paths": ["/home/user/projects", "/data"],
  "allowed_project_path": "/home/user/projects",
  "ignore_preferences": false,
  "reverse_proxy": {
    "denied_ports": [5432, 3306],
    "allowed_ports": []
  }
}
```

**Policy Fields:**
* `disabled_bricks` - Block specific tools from being installed (e.g., `["codex", "claude-code"]`)
* `enabled_bricks` - Force-enable specific tools regardless of project detection
* `allowed_mount_paths` - Whitelist of directories that can be mounted (empty = allow all except blocked)
* `allowed_project_path` - Restrict where mkenv can run (empty = allow all)
* `ignore_preferences` - Override user preferences with policy settings
* `reverse_proxy` - Control which host ports containers can access (see below)

**Policy File Security:**
* Policy files must have `0444` permissions (read-only)
* mkenv will refuse to start if policy file has incorrect permissions
* This prevents unauthorized modification of security policies

### Reverse Proxy Security

mkenv allows containers to access host services (like databases) via a reverse proxy. The policy engine strictly controls which ports can be accessed.

**Hardcoded Denied Ports (Always Blocked):**

The following ports are **permanently blocked** and cannot be overridden by policy configuration:

* **SSH & Remote Login:** 22, 2222
* **Remote Desktop:** 3389 (RDP), 5900-5903 (VNC), 5800 (VNC/HTTP), 3283 (Apple Remote Desktop)
* **File Sharing:** 445 (SMB), 139 (NetBIOS), 2049 (NFS), 111 (rpcbind)
* **Admin Panels:** 80, 443, 8080, 8443, 9090 (common admin interfaces)
* **Container Management:** 2375-2377 (Docker), 6443, 8001, 10250, 10255 (Kubernetes)
* **macOS Services:** 548 (AFP), 631 (CUPS), 5353 (mDNS)
* **Windows Services:** 135 (RPC), 593 (RPC/HTTP), 1433-1434 (MS SQL), 3268-3269 (LDAP)

**Custom Port Policies:**

```json
{
  "reverse_proxy": {
    "denied_ports": [5432, 3306],
    "allowed_ports": [8000, 8080]
  }
}
```

* `denied_ports` - Additional ports to block beyond hardcoded list
* `allowed_ports` - Explicit allowlist (if set, only these ports are accessible, but hardcoded denials still apply)

**Policy Enforcement:**
* All reverse proxy connections are checked against policy
* Denied connections are logged with source information
* All successful proxy connections are logged for auditing

### Tool Control (Bricks)

Policies can control which tools (called "bricks") are available:

```json
{
  "disabled_bricks": ["claude-code", "codex"],
  "enabled_bricks": ["nvim", "tmux", "pulumi"]
}
```

**Use Cases:**
* Block AI coding tools in security-sensitive environments
* Force-enable required development tools across teams
* Disable automatic detection and use explicit tool lists only

### Volume Mount Restrictions

```json
{
  "allowed_mount_paths": [
    "/home/user/projects",
    "/data/shared"
  ]
}
```

* If `allowed_mount_paths` is set, only directories under these paths can be mounted
* Empty list means "allow all" (except hardcoded blocked paths like `~/.ssh`)
* Relative paths are resolved before checking
* Symlink targets are checked against policy

### Project Path Restrictions

```json
{
  "allowed_project_path": "/home/user/approved-projects"
}
```

* Restricts where mkenv can be run
* Useful for isolating work projects from personal projects
* Empty string means "allow all"

### Example: Enterprise Policy

```json
{
  "disabled_bricks": ["codex"],
  "enabled_bricks": ["nvim"],
  "allowed_project_path": "/work/projects",
  "allowed_mount_paths": ["/work"],
  "ignore_preferences": true,
  "reverse_proxy": {
    "denied_ports": [5432, 3306, 6379, 27017],
    "allowed_ports": []
  }
}
```

This policy:
* Blocks OpenAI Codex but allows Neovim
* Restricts mkenv to `/work/projects` directory only
* Prevents mounting anything outside `/work`
* Blocks access to common database ports from containers
* Ignores user preferences to enforce organizational standards

---

## 🕸️ Automatic Port Forwarding

Docker doesn't allow dynamically exposing new ports on running containers. So how does `npm run dev` just work?

mkenv automatically detects and forwards ports from your container to your host - no configuration needed.

### How It Works

```
┌──────────┐
│ Browser  │
│ :3000    │
└────┬─────┘
     │
     │         ┌─────────────────────────────────────────────┐
     │         │       Your Mac (Host)                       │
     │         │                                             │
     │         │  ┌───────────────────────┐                  │
     │         │  │  mkenv host process   │    ┌──────────┐  │
     └────────────►                       │    │ Postgres │  │
               │  │  Binds :3000, :5432   │────► :5432    │  │
               │  │  on Mac on demand     │    └──────────┘  │
               │  └──────────▲────────────┘                  │
               └─────────────┼───────────────────────────────┘
                             │
                     :45454 (fixed port)
                             │
                ┌────────────▼──────────────┐
                │    mkenv Container        │
                │                           │
                │  ┌──────────────────────┐ │
                │  │  Proxy Daemon        │ │
                │  │  :45454         :5432│ │
                │  └─────────┬──────────▲─┘ │
                │            │          │   │
                │            │          │   │
                │  ┌─────────▼──────────┼─┐ │
                │  │  npm run dev :3000 │ │ │
                │  │                    │ │ │
                │  │  connects to       │ │ │
                │  │  localhost:5432 ───┘ │ │
                │  └──────────────────────┘ │
                └───────────────────────────┘

Flow (Browser → Container App):
Browser :3000 → mkenv host process (binds :3000 on Mac) →
:45454 → Proxy daemon → npm run dev :3000

Flow (Container App → Host Port):
npm run dev connects to localhost:5432 → Proxy daemon →
:45454 → mkenv host process → Postgres :5432 on Mac

The mkenv host process is the traffic hub. All localhost traffic
routes through the fixed :45454 port to avoid Docker limitations.
```

The mkenv host process acts as a bidirectional traffic hub between your Mac and the container, using a fixed port (:45454) to avoid Docker's dynamic port limitation.

- **Browser → Container:** Your browser accesses localhost:3000, which the host process routes to your containerized app
- **Container → Host:** Your containerized app accesses localhost:5432, which routes through the host process to any app on your Mac

Ports are bound **on demand dynamically**. No manual configuration needed.

**Bonus:** Since all traffic flows through the host process, mkenv logs every connection for audit and analysis. All logging happens offline on your machine - nothing leaves your laptop. This gives you visibility into what your container is actually doing.

### Additional Details

* Supports TCP and optionally UDP listeners
* Works even if your dev server restarts or changes ports
* Minimal overhead - both forwarder and proxy are lightweight
* Handles multiple services on multiple ports automatically
* Warns gracefully if a host port is already in use
* Provides proper `EADDRINUSE` errors matching native behavior
* All traffic is logged for auditing

## 📦 Installation

### Homebrew (Recommended)

```sh
brew tap 0xa1bed0/mkenv
brew install mkenv
```

### Direct Downloads

**Latest stable:**
```
https://github.com/0xa1bed0/mkenv/releases/download/latest/mkenv-darwin-arm64
```

**Specific version:**
```
https://github.com/0xa1bed0/mkenv/releases/download/vX.Y.Z/mkenv-darwin-arm64
```

**Checksums:**
```
https://github.com/0xa1bed0/mkenv/releases/download/latest/checksums.txt
```

---

## 🧭 Philosophy

* **Minimal attack surface** — tight guardrails, deterministic builds
* **Zero config** for most users
* **Maximum observability** — inspect container, inspect build, inspect Dockerfile
* **Developer-first** — optimized for CLI/terminal workflows
* **Performance** — build & run in seconds

---

## 📝 Roadmap

### 🔧 Short-Term (Next Steps)

* Support for more languages, tools, and frameworks
* In-container management tool for runtime changes
* Internal refactoring and full test coverage
* Network lockdown mode
* Network traffic auditing (inbound and outbound)
* Safe dependency installation during build 

### 🌱 Long-Term Vision

* A universal, zero‑config dev environment engine
* Reproducible, secure, disposable workspaces for every project
* Enterprise features for teams

---

## 🤝 Contributing

`mkenv` is in active development. Bug reports, feature ideas, and PRs are extremely welcome!

---

## 📄 License

mkenv is licensed under the **Elastic License 2.0 (ELv2)**.

### ✔ What you are allowed to do
- Use mkenv **for free**, personally or professionally  
- Use it at work, individually or across a whole company  
- Modify it, fork it, and redistribute it  
- Use it in internal tooling  
- Use it in CI/CD, development, local environments, etc.  
- Build your own non-commercial forks or experiments  

mkenv is **free for all real-world usage**, including organizational usage.

### ❌ What you are **not** allowed to do
- Sell mkenv  
- Sell a modified version of mkenv  
- Offer mkenv as a hosted/managed service (SaaS)  
- Embed mkenv into a commercial product  
- Build or sell a commercial competitor based on mkenv  
