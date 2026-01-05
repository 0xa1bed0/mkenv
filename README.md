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

Works with VS Code, Cursor, Neovim, Tmux, and other tools. Zero configuration required.

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

**Coming soon:** VS Code and Cursor extensions for seamless container integration.

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

### **Use with VS Code**

If the extension is installed:

* Opening a folder triggers an mkenv container if one exists
* You can open terminals directly inside the container

### **Use with Cursor**

The Cursor extension ensures:

* All agent commands run inside the mkenv container
* Terminals default to the dev environment

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

mkenv ensures safety before you start:

* Scans project files before creating containers and warns if secrets are detected
* Supports folder whitelists to restrict where mkenv can run
* Allows allowlists for volume mounts to prevent mounting sensitive directories
* Automatically blocks dangerous folders like `~/.ssh` from being mounted (cannot be overridden)

---

## 🕸️ Automatic Port Forwarding

Docker doesn't allow dynamically exposing new ports on running containers. So how does `npm run dev` just work?

mkenv automatically detects and forwards ports from your container to your host - no configuration needed.

### How It Works

1. When you start mkenv, a lightweight forwarder runs on your host
2. Inside the container, a daemon monitors for new listening sockets
3. When your app binds to a port (e.g., `npm run dev` on port 3000), the daemon detects it
4. The forwarder automatically opens the corresponding host port and routes traffic to the container
5. Requests are forwarded to your application seamlessly

**Result:** Ports just work. Zero configuration. No Docker quirks.

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
* VS Code & Cursor extensions
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
