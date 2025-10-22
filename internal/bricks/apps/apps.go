package apps

import "github.com/0xa1bed0/mkenv/internal/dockerimage"

type App interface {
	dockerimage.Brick
	// SetVersion sets the app version manually.
	SetVersion(version string) 
	GetVersion() string
}

type AppBrickBase struct {
	dockerimage.BrickBase
}

func (app *AppBrickBase) SetVersion(version string) {
	app.SetParam("version", version)
}

func (app *AppBrickBase) GetVersion() string {
	version, _ := app.GetParam("version")
	return version
}
