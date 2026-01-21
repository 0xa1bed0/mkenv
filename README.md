> [!WARNING]
> **Early Stage Software:** mkenv is experimental. Expect breaking changes, incomplete features, and occasional instability. We're releasing early to gather feedback.

# mkenv

[![Website](https://img.shields.io/badge/website-mkenv.sh-blue)](https://mkenv.sh)
[![License: Elastic License 2.0](https://img.shields.io/badge/license-ELv2-blue)](#license)

**Sandboxed dev environments that feel like localhost.**

mkenv creates disposable Docker containers for your project. No port mapping — ports bind dynamically. No `host.docker.internal` — localhost just works. No Dockerfile to write — mkenv auto-detects your stack. Sensitive paths like `~/.ssh` and `~/.aws` are blocked from mounting.

```sh
cd ~/projects/myapp && mkenv .
```

Your `npm install`, `pip install`, LLM agents, and builds run isolated. Your credentials stay on your host.

## Install

**Prerequisites:** Docker Desktop or Docker Engine

**macOS (Homebrew):**
```sh
brew tap 0xa1bed0/mkenv
brew install mkenv
```

**Linux / Windows (WSL):**
```sh
# Download binary (arm64 or amd64)
curl -L https://github.com/0xa1bed0/mkenv/releases/latest/download/mkenv-linux-arm64 -o mkenv
# or: curl -L https://github.com/0xa1bed0/mkenv/releases/latest/download/mkenv-linux-amd64 -o mkenv

chmod +x mkenv
sudo mv mkenv /usr/local/bin/
```

**Platform support:** macOS, Linux, and Windows (via WSL).

## Features

- **Auto-detection** — Scans your project, detects runtimes (Node, Go, etc.), builds environment automatically
- **Isolated execution** — Your LLM agents and package installs run inside the sandbox, not on your host
- **Localhost behavior** — No port configuration, no `host.docker.internal`. Ports bind dynamically on demand
- **Blocked sensitive paths** — Can't mount `~/.ssh`, `~/.aws`, `~/.docker`, browser profiles, password managers
- **Pre-flight secret scan** — Scans your project for `.env` files, API keys, private keys before starting
- **Network audit** — All traffic logged locally. System-critical ports blocked by default
- **Policy engine** — Protects devs from accidental mistakes. Teams can enforce their own rules

### Hardened by default

- Runs as non-root user
- Shell history never stores tokens, keys, secrets
- Critical paths physically blocked from mounting
- Container destroyed on exit — compromised state can't persist
- Dangerous actions require explicit approval

## How is this different?

**vs Devcontainers:** No Dockerfile or devcontainer.json to write. Ports bind dynamically without configuration. Security guardrails are built-in — you can't accidentally mount `~/.ssh` even if you try.

**vs plain Docker:** With Docker you build and maintain your environment from scratch, learning security quirks along the way. mkenv auto-detects your project, handles port forwarding bidirectionally, and bakes in security defaults.

**vs Nix/Devbox:** Those manage your toolchain but run code on your host. mkenv runs everything in a container — actual isolation, not just reproducibility.

**Policy engine:** mkenv has a built-in policy engine. Developers are protected from accidental mistakes (mounting credentials, exposing dangerous ports). Teams can enforce their own rules. None of the alternatives have this built-in.

**Audit trail:** mkenv logs everything — network connections, port bindings, package installs, system changes. When something goes wrong, you know exactly what happened.

## Configuration (optional)

Most projects need zero configuration. When you do need to customize:

```json
// .mkenv in project root
{
  "enabled_bricks": ["claude-code", "nvim"],
  "volumes": ["~/data:/data"]
}
```

Bricks are atomic building blocks — things like `claude-code`, `nvim`, `node`, `go`.

For security policies enforcement, see [policy documentation](https://mkenv.sh/docs.html#policies).

## Documentation

Full documentation at **[mkenv.sh/docs.html](https://mkenv.sh/docs.html)**

- [Dynamic Port forwarding](https://mkenv.sh/docs.html#port-forwarding) — How bidirectional localhost works
- [Configuration files](https://mkenv.sh/docs.html#mkenv-files) — Optional .mkenv file reference
- [Security policies](https://mkenv.sh/docs.html#policies) — Enforce security guardrails
- [FAQ](https://mkenv.sh/docs.html#faq) — Git push, sudo packages, IDE integration

## License

**Elastic License 2.0 (ELv2)**

**You can:** Use mkenv for personal projects, at work, across your company, in CI pipelines, and modify it for internal use.

**You cannot:** Resell mkenv, offer it as a hosted service (SaaS), or build a competing product from it.

## Contributing

Bug reports, feature ideas, and PRs welcome. Open an issue at [github.com/0xa1bed0/mkenv/issues](https://github.com/0xa1bed0/mkenv/issues).
