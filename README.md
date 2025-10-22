# mkenv

mkenv is a zero-config CLI that inspects your project and prints a ready-to-run
devcontainer Dockerfile. It assembles small, reusable “bricks” for the base
operating system, your preferred shell, and detected language runtimes, so you
can bootstrap a containerised development environment with a single command.

## Why mkenv?

- **One-liner**: `mkenv ./my-project` starts container with all necessary tools
- **Language auto-detection**: finds language-specific tooling (currently Go)
  without additional configuration.
- **Composable bricks**: system, shell, and language layers stay isolated and
  easy to extend.

## Installation

mkenv is distributed as pre-built binaries no local Go toolchain required.

### Homebrew (macOS and Linux)

```sh
brew install 0xa1bed0/tap/mkenv
```

### Debian/Ubuntu (APT)

```sh
echo "deb [trusted=yes] https://packages.mkenv.sh/mkenv stable main" | sudo tee /etc/apt/sources.list.d/mkenv.list
sudo apt update
sudo apt install mkenv
```

### Manual download

1. Grab the latest release for your platform from the GitHub Releases page.
2. Make the binary executable: `chmod +x mkenv`.
3. Place it anywhere on your `PATH`, e.g. `mv mkenv /usr/local/bin/`.

### Build from source (optional)

```sh
git clone https://github.com/0xa1bed0/mkenv.git
cd mkenv
go build -o mkenv .
```

## Usage

```sh
mkenv /path/to/project
```

Flags:

- `--editor` – optionally record your preferred editor in future workflows
- `--lang` – force-enable a language brick (future extension)

## How it works

1. The CLI creates a `FileManager` rooted at the provided path.
2. Language bricks inspect the project (e.g. search for `go.mod`) and report
   whether their tooling is needed.
3. The `dockerimagebuilder` composes the base system (Debian + Oh My Zsh) with
   all enabled language bricks and prints the combined Dockerfile patch.

## Contributing

1. Fork the repository and create a feature branch.
2. Run `go test ./...` before opening a pull request.
3. Document new bricks or behaviours in this README.

Issues and ideas are welcome let us know what additional languages or shells
you want to see supported!
