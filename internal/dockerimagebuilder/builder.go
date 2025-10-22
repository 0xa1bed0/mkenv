// Package dockerimagebuilder composes system and language bricks into a single
// Dockerfile definition.
package dockerimagebuilder

import (
	"github.com/0xa1bed0/mkenv/internal/bricks/languages"
	"github.com/0xa1bed0/mkenv/internal/bricks/systems"
	"github.com/0xa1bed0/mkenv/internal/bricks/systems/shells"
	"github.com/0xa1bed0/mkenv/internal/dockerimage"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

type image struct {
	bricks []dockerimage.Brick

	from string
	buildShell []string
	runAsRoot []string
	runAsUser []string
	envs map[string]string
	workdir string
	cmd []string
}

// AddBrick implements Image.
func (i *image) AddBrick(brick dockerimage.Brick) error {
	i.bricks = append(i.bricks, brick)
	return nil
}

// Compile implements Image.
func (i *image) Compile() (string, error) {
	dockerfile := ""

	for _, brick := range i.bricks {
		patch, err := brick.GetDockerfilePatch()
		if err != nil {
			return "", err
		}
		dockerfile += patch
	}

	return dockerfile, nil
}

// NewImage constructs an image populated with the base system brick and any
// language bricks that match the project code.
func NewImage(projectFolderPtr filesmanager.FileManager) (dockerimage.Image, error) {
	image := &image{
		bricks:     []dockerimage.Brick{},
		from:       "",
		buildShell: []string{"/bin/bash", "-lc"},
		runAsRoot:  []string{},
		runAsUser:  []string{
			"mkdir -p $HOME/workspace",
		},
		workdir: "$HOME/workspace",
		envs:       map[string]string{
			"USER_NAME": "dev",
			"USER_UID": "1000",
			"USER_GID": "1000",
			"HOME": "/home/${USER_NAME}",
		},
		cmd:        []string{},
	}

	ohmyzsh := shells.NewShellOhMyZsh()

	system := systems.NewSystemDebianWithShell(ohmyzsh)

	image.AddBrick(system)
	// TODO: should we add briks blindly? some briks may not change some of the builder parameters (like runAsRoot or 
	// override existing envs) maybe we should understand which brick type can do what and not store them as generic but 
	// something more clever. Also consider user config for everything.

	langbricks, err := languages.GetEnabledLangBricks(projectFolderPtr)
	if err != nil {
		return nil, err
	}

	for _, lang := range langbricks {
		image.AddBrick(lang)
	}

	return image, nil
}
