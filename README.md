# mkenv

[![License: Elastic License 2.0](https://img.shields.io/badge/license-Elastic%20License%202.0-blue)](#license)
[![Contributions Welcome](https://img.shields.io/badge/contributions-welcome-brightgreen)](../../issues)
![Status: Active](https://img.shields.io/badge/status-active-success)
![Free for personal and commercial internal use](https://img.shields.io/badge/free%20for-personal%20%2B%20company%20use-green)
![Not for SaaS or resale](https://img.shields.io/badge/restrictions-no%20SaaS%20%2F%20no%20resale-red)

**mkenv** is a fast, secure, reproducible development environment generator. It creates fully isolated Docker-based dev environments in seconds — complete with your editor, tools, configs, caches, and volumes — without cluttering your host system.

`mkenv` integrates naturally with VS Code, Cursor, Neovim, Tmux, and other tools. It's designed for developers who want reproducible dev setups without Docker Compose bloat, without Nix complexity, and without leaking host state into containers.

---

## 🚀 Key Features

### **Zero‑Config Environment Detection (the core feature)**

`mkenv` automatically analyzes your project and **estimates exactly which development environment you need**.

* Detects languages (Go, NodeJS, Python, Rust, etc.)
* Infers toolchain versions from files like `go.mod`, `brickage.json`, `.nvmrc`, etc.
* Picks correct Bricks (language/tool building blocks)
* Generates a sophisticated Docker environment **without a single configuration file**

This is the main differentiator from Devcontainers, Nix, or Compose setups.

### **Instant Dev Environments**

```sh
mkenv .
```

Creates a full Docker dev environment with:

* Auto‑generated Dockerfile (built from Bricks)
* Proper language runtimes & dev tools
* Auto-mounted project
* Cached dependencies & build artifacts
* Editor configs (optional)
* Tmux + zsh environment

### **Disposable Containers, Persistent Everything Else**

Containers are meant to be temporary — removed between sessions.
But `mkenv` persists everything you care about:

* zsh history
* Tmux configs
* Neovim configs
* Language build caches
* Dependency caches (`npm`, `go mod`, `pip`, …)

You get **fresh container, warm caches**.

### **Bricks: Modular Building Blocks**

Bricks are small declarative units describing:

* Apt packages
* User commands
* Environment variables
* Build steps
* Version detection logic

`mkenv` assembles Bricks into a full Dockerfile automatically.

### **Editor Integration (in progress)**

VS Code & Cursor extensions will:

* Automatically open the project’s container
* Run terminals inside mkenv
* Run AI agents inside the mkenv environment

### **Actively in Development**

The tool already supports:

* Golang
* NodeJS
* Debian base system
* Neovim
* Tmux
* Core user workflow

Feedback & contributions are very welcome.

---

## 📦 What Problems mkenv Solves

### **No more global clutter**

No global Node, Go, Python, Rust, Bun, Deno, pnpm, etc.

### **Fully isolated dev machines**

Every project runs in its own sandbox.

### **Reproducible and debuggable**

Your workspace image is built deterministically:

* Dockerfile assembled from Bricks (system + language + common)
* Hash-based cache keys ensure repeatable image builds
* Build caches and volumes are isolated per project

### **Fast rebuilds**

Heavy dependencies are cached across rebuilds.

### **Editor integration**

VS Code and Cursor extensions (optional) allow:

* Opening the project’s dev container automatically
* Executing agent commands inside the container
* Running terminals inside the container by default

---

## 🧠 How It Works

### **1. Compose a Workspace Image**

`mkenv` determines what language bricks to enable based on your folder:

* Go
* NodeJS
* Python
* Rust
* etc.

Each brick contributes:

* Apt packages
* Shell commands
* Build arguments
* User-level commands
* Environment variables

The system brick defines:

* `FROM` base image
* system-level arguments (username, uid/gid)
* entrypoint + cmd guardrails

Everything is assembled into a deterministic Dockerfile.

### **2. Run the Dev Container**

Your container gets a deterministic name:

```
home_-projects-myapp-123abc
```

* Derived from the absolute path of your project
* Home folder is replaced with `home_`
* Truncated automatically when too long

### **3. Open an interactive shell**

The container starts tmux/zsh by default.
Resizes dynamically with your terminal.

---

## 🛠 Usage

### **Create / enter environment:**

```sh
mkenv .
```

### **Leave environment:**

Just exit the shell or tmux session.
By default the container is removed.

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

mkenv automatically creates and reuses volumes.
You can define additional cache paths in your language brick.

---

## 🔐 Security Considerations

* All system-level commands are guarded and executed as root only during image build
* All brick “common” commands run as user inside the container
* No arbitrary user overrides of entrypoint or system args
* The cache file is considered untrusted input; mkenv validates and rewrites it
* Node/Golang/etc dependencies are installed *inside the workspace image*, never touching the host

---

## 🧩 Bricks (Building Blocks)

Bricks are declarative modules describing:

```go
type CommonBrick struct {
    Env map[string]string
    Apt []string
    UserCommands []string
    DockerArgs map[string]string
}

type SystemBrick struct {
    From string
    BuildShell []string
    SystemArgs map[string]string
    // guards: cannot be overridden
}
```

You can ship:

* system brick (exactly one)
* multiple language bricks (auto-detected)
* multiple common bricks (merged)

---

## 🔄 Deterministic Cache Keys

Cache keys use:

* Dockerfile lines with prefixed lengths
* Bricked sorted arguments
* Short stable SHA256 identifiers

This ensures rebuilds happen only when something truly changes.

---

## 📦 Installation

Coming soon — Homebrew formula & direct binary downloads.

For now - navigate to the Github Releases page and download the pre-build binary

or install it from source:

```
go install github.com/0xa1bed0/mkenv@latest
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

* Add more Bricks (languages, tools, frameworks, LLM runtimes)
* Improve global UX: user preferences, project configs, logging
* In‑container management tool (ephemeral installs, runtime changes)
* Internal refactoring + full test coverage
* VS Code & Cursor extensions
* Security policies & sandbox hardening

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

### 🏢 mkenv Enterprise
A separate **closed-source enterprise edition** is available from  
**Albedo Technologies SRL**  
with additional features (e.g., team policies, security controls, orchestration).

The enterprise edition is a separate product and not part of this repository.

