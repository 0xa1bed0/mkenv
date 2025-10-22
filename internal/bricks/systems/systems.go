// Package systems defines devcontainer bricks that configure the base operating
// system and shell inside the container.
package systems

import (
	"github.com/0xa1bed0/mkenv/internal/bricks/systems/shells"
	"github.com/0xa1bed0/mkenv/internal/dockerimage"
)

// System is a brick that manages OS-level setup for the container and can be
// extended with shell configuration.
type System interface {
	dockerimage.Brick
	SetShell(shell shells.Shell)
}

type SystemBase struct {
	dockerimage.BrickBase
	shell shells.Shell
}

func (systemBase *SystemBase) SetShell(shell shells.Shell) {
	systemBase.SetParam("shell", shell.GetName())

	systemBase.shell = shell
}
