package langs

import (
	"strings"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

const (
	ocamlID          = bricksengine.BrickID("ocaml")
	ocamlDescription = "OCaml toolchain"
)

var ocamlKinds = []bricksengine.BrickKind{bricksengine.BrickKindCommon}

func NewOCaml(metadata map[string]string) (bricksengine.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	version, ok := metadata["version"]
	if !ok || version == "" {
		version = "5.2.0"
	} else {
		version = strings.TrimPrefix(version, "ocaml")
		version = strings.TrimPrefix(version, "ocaml-base-compiler.")
	}

	brick, err := bricksengine.NewBrick(ocamlID, ocamlDescription,
		bricksengine.WithKinds(ocamlKinds),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "opam and OCaml build dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "opam"},
				{Name: "curl"},
				{Name: "ca-certificates"},
				{Name: "git"},
				{Name: "build-essential"},
				{Name: "m4"},
				{Name: "pkg-config"},
				{Name: "unzip"},
				{Name: "rsync"},
				{Name: "bubblewrap"},
				{Name: "libgmp-dev"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.opam"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.cache/dune"),
		bricksengine.WithEnv("OPAMROOT", "${MKENV_HOME}/.opam"),
		bricksengine.WithEnv("OPAMYES", "1"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{
				"/bin/bash", "-lc", `set -eo pipefail
export OCAML_VERSION=` + version + `

# opam sandboxing relies on bubblewrap, which is unavailable inside the
# container, so it is disabled here.
opam init --bare --disable-sandboxing --yes

# Create the default switch with the requested compiler. Fall back to the
# bare version selector if the base-compiler package name is unavailable.
opam switch create default ocaml-base-compiler.$OCAML_VERSION --yes \
  || opam switch create default $OCAML_VERSION --yes

eval "$(opam env --switch=default --set-switch)"

# Common OCaml development tooling.
opam install --yes dune merlin ocaml-lsp-server ocamlformat utop odoc

# Expose the toolchain on MKENV_LOCAL_BIN so it is on PATH without an opam env.
for tool in opam dune ocaml ocamlc ocamlfind ocamllsp ocamlformat utop merlin; do
  if command -v "$tool" >/dev/null 2>&1; then
    ln -sf "$(command -v "$tool")" "${MKENV_LOCAL_BIN}/$tool" 2>/dev/null || true
  fi
done
`,
			},
		}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "lang/ocaml",
			FilePath: "rc",
			Content: `# OCaml opam environment start
export OPAMROOT="${OPAMROOT:-$HOME/.opam}"
command -v opam >/dev/null 2>&1 && eval "$(opam env 2>/dev/null)" || true
# OCaml opam environment end`,
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

type ocamlDetector struct {
	langDetector bricksengine.LangDetector
}

func (*ocamlDetector) BrickInfo() *bricksengine.BrickInfo {
	return bricksengine.NewBrickInfo(ocamlID, ocamlDescription, ocamlKinds)
}

func (od *ocamlDetector) Scan(folderPtr filesmanager.FileManager) (bricksengine.BrickID, map[string]string, error) {
	found, brickMeta, err := od.langDetector.ScanFiles(folderPtr)
	if err != nil {
		return "", nil, err
	}
	if found {
		return ocamlID, brickMeta, nil
	}
	return "", nil, nil
}

func init() {
	bricksengine.RegisterBrick(ocamlID, NewOCaml)
	bricksengine.RegisterDetector(func() bricksengine.BrickDetector {
		return &ocamlDetector{
			langDetector: bricksengine.NewLangDetector(string(ocamlID), "dune-project", "ml,mli", ""),
		}
	})
}
