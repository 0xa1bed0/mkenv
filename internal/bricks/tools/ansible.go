package tools

import "github.com/0xa1bed0/mkenv/internal/bricksengine"

const ansible = "ansible"

func NewAnsible(metadata map[string]string) (bricksengine.Brick, error) {
	brick, err := bricksengine.NewBrick(ansible, "Ansible automation platform",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "Ansible dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "python3"},
				{Name: "python3-pip"},
				{Name: "python3-venv"},
				{Name: "sshpass"},
			},
		}),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.cache/pip"),
		bricksengine.WithCacheFolder("${MKENV_HOME}/.ansible"),
		bricksengine.WithUserRun(bricksengine.Command{
			When: "build",
			Argv: []string{"/bin/bash", "-lc", `set -eo pipefail && ` +
				`pip3 install --user --break-system-packages ansible && ` +
				`for bin in ansible ansible-playbook ansible-galaxy ansible-vault ansible-config ansible-inventory ansible-doc; do ` +
				`ln -sf "${MKENV_HOME}/.local/bin/${bin}" "${MKENV_LOCAL_BIN}/${bin}"; ` +
				`done`},
		}),
	)
	if err != nil {
		return nil, err
	}
	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(ansible, NewAnsible)
}
