# Contributing to mkenv

Thank you for considering contributing to **mkenv**!

mkenv is developed and maintained by **Albedo Technologies SRL** and released under the **Elastic License 2.0 (ELv2)**. The core is free for all personal and organizational internal use, while commercial resale and SaaS usage are restricted. A separate proprietary enterprise edition exists outside this repository.

This document outlines how to contribute safely and effectively.

---

## 🧱 Project Philosophy

mkenv’s goal is to provide a **zero-configuration, automatically generated development environment engine**. The project values:

* **Simplicity and clarity** — small, composable building blocks (Bricks)
* **Automatic detection** of languages, tools, and versions
* **Determinism** — reproducible environments
* **Security** — clear boundaries between OSS and enterprise features
* **Practicality** — developer-centric workflows

---

## 🪛 How You Can Contribute

### 1. Issues

If you encounter a bug, have a feature request, or want to discuss a design idea:

* Search existing issues first
* Open a new issue with clear reproduction steps or context

Good issues include environment info, terminal output, logs, or small reproducing repos.

---

### 2. Pull Requests

PRs are welcome for:

* Bug fixes
* Performance improvements
* Reliability or safety improvements
* Documentation improvements
* New language/tool **Bricks** (detectors, Dockerfile sections, metadata)
* Refactoring that improves maintainability

PRs that are *not* accepted:

* Enterprise-only features (team orchestration, policy engines, centralized management)
* Features that weaken the security or isolation model
* Integrations that depend on proprietary services
* Licensing or trademark changes

Before opening a large PR, consider creating a Discussion or Issue first.

---

## 🧪 Code Guidelines

* Use Go best practices
* Keep Bricks small, composable, and side-effect-free
* Maintain deterministic behavior (inputs → same outputs)
* Prefer explicitness over magic
* Add tests when possible (tests may expand after POC phase)
* Follow existing patterns for CLI, Dockerfile generation, and caching

Structure should support future language/tool Bricks without breaking the architecture.

---

## 🔐 Licensing of Contributions

By contributing to this repository, you agree that:

1. **Your contributions are licensed under Elastic License 2.0 (ELv2)**.
2. You grant **Albedo Technologies SRL** a perpetual right to use your contribution in:

   * the open-source mkenv version
   * proprietary enterprise editions
   * derivative works
3. You confirm that your contribution is your original work and does not violate third-party IP.

This is standard for projects with an open-source core and a proprietary enterprise layer.

---

## 🌱 Community Contributions

This project welcomes contributions from:

* Individual developers
* Companies using mkenv internally
* Hobbyists and tool enthusiasts

mkenv is early in its evolution, so contributors who help shape the architecture will have significant impact.

---

## ❤️ Thanks

We appreciate every contributor who helps make mkenv a powerful, reliable, zero-config development environment tool.

If you’re unsure where to start, check the issues tagged **good first issue** or open a discussion!

