package systems

import (
	"github.com/0xa1bed0/mkenv/internal/bricks/systems/shells"
)

type debian struct {
	SystemBase
}

// GetDockerfilePatch implements container.Brick.
func (d *debian) GetDockerfilePatch() (string, error) {
	patch := `FROM debian:bookworm-slim
SHELL ["/bin/bash", "-lc"]

ARG DEBIAN_FRONTEND=noninteractive

ARG USER_NAME=dev
ARG USER_UID=1000
ARG USER_GID=1000

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
	ca-certificates \
	curl \
	git \
	less \
	build-essential \
	pkg-config \
	cmake \
	&& rm -rf /var/lib/apt/lists/*

RUN groupadd --gid ${USER_GID} ${USER_NAME} \
	&& useradd --uid ${USER_UID} --gid ${USER_GID} -m ${USER_NAME}

USER ${USER_NAME}
RUN mkdir /workspace
WORKDIR /workspace
`

	shellConf, err := d.shell.GetDockerfilePatch()
	if err != nil {
		return "", err
	}

	patch += shellConf

	patch += `
USER ${USER_NAME}
ENV HOME=/home/${USER_NAME}
`

	return patch, nil
}

// NewSystemDebian creates a Debian-based system brick configured with the
// default (zsh) shell
func NewSystemDebian() System {
	return NewSystemDebianWithShell(shells.NewShellZSH())
}

// NewSystemDebianWithShell creates a Debian-based system brick configured with the
// provided shell.
func NewSystemDebianWithShell(shell shells.Shell) System {
	d := &debian{ }
	if shell != nil {
		d.SetShell(shell)
	}

	return d
}
