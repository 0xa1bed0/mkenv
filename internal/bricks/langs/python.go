package langs

import (
	"strings"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

const (
	pythonID          = bricksengine.BrickID("python")
	pythonDescription = "Python toolchain"
)

var pythonKinds = []bricksengine.BrickKind{bricksengine.BrickKindCommon}

func NewPython(metadata map[string]string) (bricksengine.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	version, ok := metadata["version"]
	if !ok || version == "" {
		version = "3.12"
	} else {
		version = strings.TrimPrefix(version, "python")
		version = strings.TrimPrefix(version, "py")
	}

	brick, err := bricksengine.NewBrick(pythonID, pythonDescription,
		bricksengine.WithKinds(pythonKinds),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "pyenv install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "curl"},
				{Name: "ca-certificates"},
				{Name: "git"},
				{Name: "build-essential"},
				{Name: "libssl-dev"},
				{Name: "zlib1g-dev"},
				{Name: "libbz2-dev"},
				{Name: "libreadline-dev"},
				{Name: "libsqlite3-dev"},
				{Name: "libncursesw5-dev"},
				{Name: "xz-utils"},
				{Name: "tk-dev"},
				{Name: "libxml2-dev"},
				{Name: "libxmlsec1-dev"},
				{Name: "libffi-dev"},
				{Name: "liblzma-dev"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.cache/pip"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.pyenv/versions"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.local/share/virtualenvs"),
		bricksengine.WithEnv("PYENV_ROOT", "${MKENV_HOME}/.pyenv"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{
				"/bin/bash", "-lc", `set -eo pipefail
export PYTHON_VERSION=` + version + `
curl -fsSL https://pyenv.run | bash
export PATH="$PYENV_ROOT/bin:$PATH"
eval "$(pyenv init -)"

# Find latest matching version
available=$(pyenv install --list | tr -d ' ' | grep -E "^${PYTHON_VERSION}(\.[0-9]+)?$" | tail -n1 || true)
if [ -z "$available" ]; then
  available=$(pyenv install --list | tr -d ' ' | grep -E "^${PYTHON_VERSION%%.*}\.[0-9]+\.[0-9]+$" | tail -n1)
fi
if [ -z "$available" ]; then
  echo "Could not find Python version matching: $PYTHON_VERSION"
  exit 1
fi

PYTHON_VERSION="$available"
pyenv install "$PYTHON_VERSION"
pyenv global "$PYTHON_VERSION"

# Install common Python development tools
pip install --upgrade pip setuptools wheel
pip install pipx
pipx ensurepath

# Install common development tools via pipx (isolated environments)
pipx install poetry
pipx install pipenv
pipx install black
pipx install flake8
pipx install mypy
pipx install pytest
pipx install ruff
pipx install isort
pipx install pre-commit
pipx install httpie
pipx install ipython

# Create symlinks in MKENV_LOCAL_BIN
for tool in poetry pipenv black flake8 mypy pytest ruff isort pre-commit http https ipython; do
  if command -v "$tool" &>/dev/null; then
    ln -sf "$(command -v $tool)" "${MKENV_LOCAL_BIN}/$tool" 2>/dev/null || true
  fi
done

# Ensure python and pip are accessible
ln -sf "$(pyenv which python)" "${MKENV_LOCAL_BIN}/python"
ln -sf "$(pyenv which pip)" "${MKENV_LOCAL_BIN}/pip"
`,
			},
		}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "lang/python",
			FilePath: "rc",
			Content: `# Python version manager start
export PYENV_ROOT="${PYENV_ROOT:-$HOME/.pyenv}"
[ -d "$PYENV_ROOT/bin" ] && export PATH="$PYENV_ROOT/bin:$PATH"
command -v pyenv &>/dev/null && eval "$(pyenv init -)"
# Python version manager end`,
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

type pythonDetector struct {
	langDetector bricksengine.LangDetector
}

func (*pythonDetector) BrickInfo() *bricksengine.BrickInfo {
	return bricksengine.NewBrickInfo(pythonID, pythonDescription, pythonKinds)
}

func (pd *pythonDetector) Scan(folderPtr filesmanager.FileManager) (bricksengine.BrickID, map[string]string, error) {
	found, brickMeta, err := pd.langDetector.ScanFiles(folderPtr)
	if err != nil {
		return "", nil, err
	}
	if found {
		return pythonID, brickMeta, nil
	}
	return "", nil, nil
}

func init() {
	bricksengine.RegisterBrick(pythonID, NewPython)
	bricksengine.RegisterDetector(func() bricksengine.BrickDetector {
		return &pythonDetector{langDetector: bricksengine.NewLangDetector(string(pythonID), "requirements.txt,pyproject.toml,setup.py,Pipfile", "py", `python_requires`)}
	})
}
