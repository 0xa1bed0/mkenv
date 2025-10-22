// Package dockerimage defines the minimal interfaces required to compose
// reusable Dockerfile fragments.
package dockerimage

import (
	"fmt"
)

// Brick is an atomic part of a Docker image. Many bricks combine into a single
// final image definition.
type Brick interface {
	// Name returns the identifier of the brick.
	GetName() string
	SetName(name string) error
	// GetDockerfilePatch generates a Dockerfile fragment for this brick.
	GetDockerfilePatch() (string, error)
	// SetParam configures the brick; repeated calls override previous values.
	SetParam(key, value string) 
	GetParam(key string) (string, bool)
}

type BrickBase struct {
	params map[string]string
}

func (brickBase *BrickBase) GetName() string {
	name, _ := brickBase.GetParam("name")
	return name
}

func (brickBase *BrickBase) SetName(name string) error {
	existingName, nameAlreadySet := brickBase.GetParam("name")
	if nameAlreadySet {
		return fmt.Errorf("Name for brick %s already set", existingName)
	}
	
	brickBase.SetParam("name", name)
	return nil
}

func (brickBase *BrickBase) SetParam(key, value string) {
	if brickBase.params == nil {
		brickBase.params = make(map[string]string)
	}

	brickBase.params[key] = value
}

func (brickBase *BrickBase) GetParam(key string) (string, bool) {
	if brickBase.params == nil {
		brickBase.params = make(map[string]string)
		return "", false
	}
	value, ok := brickBase.params[key]
	return value, ok
}

// Image represents a collection of bricks that can be compiled into a
// Dockerfile.
type Image interface {
	AddBrick(brick Brick) error
	Compile() (string, error)
}
