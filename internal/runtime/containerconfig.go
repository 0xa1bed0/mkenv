package runtime

import (
	"context"
	"sync"

	"github.com/0xa1bed0/mkenv/internal/logs"
)

type containerConfigOnChangeHandler func()

type ContainerConfig struct {
	imageTag string
	id       string
	port     int

	stopContainer context.CancelFunc

	mu               sync.Mutex
	onChangeHandlers []containerConfigOnChangeHandler
}

func (cc *ContainerConfig) StopContainer() {
	if cc.stopContainer != nil {
		cc.stopContainer()
		cc.stopContainer = nil
	}
}

func (cc *ContainerConfig) ImageTag() string {
	return cc.imageTag
}

func (cc *ContainerConfig) ContainerID() string {
	return cc.id
}

func (cc *ContainerConfig) Port() int {
	return cc.port
}

func (cc *ContainerConfig) OnChange(fn containerConfigOnChangeHandler) int {
	cc.mu.Lock()
	var idx int
	cc.onChangeHandlers = append(cc.onChangeHandlers, fn)
	idx = len(cc.onChangeHandlers)
	cc.mu.Unlock()
	return idx
}

func (cc *ContainerConfig) RemoveOnChangeHandler(idx int) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if idx < 0 || idx >= len(cc.onChangeHandlers) {
		return
	}
	cc.onChangeHandlers[idx] = nil
	cc.onChangeHandlers = append(cc.onChangeHandlers[:idx], cc.onChangeHandlers[idx+1:]...)
}

func (cc *ContainerConfig) SetStopContainer(fn context.CancelFunc) {
	if cc.stopContainer != nil {
		cc.stopContainer = nil
	}
	cc.stopContainer = fn
}

func (cc *ContainerConfig) SetImageTag(imageTag string) {
	cc.mu.Lock()

	cc.imageTag = imageTag
	onChangeHandlers := make([]containerConfigOnChangeHandler, len(cc.onChangeHandlers))
	copy(onChangeHandlers, cc.onChangeHandlers)

	cc.mu.Unlock()

	if len(onChangeHandlers) > 0 {
		// Recover so the callback cannot break Runtime.
		defer func() {
			if rec := recover(); rec != nil {
				logs.Errorf("on container change hook failed: %v", rec)
			}
		}()

		for _, h := range onChangeHandlers {
			h()
		}
	}
}

func (cc *ContainerConfig) SetContainerID(containerID string) {
	cc.mu.Lock()

	cc.id = containerID
	onChangeHandlers := make([]containerConfigOnChangeHandler, len(cc.onChangeHandlers))
	copy(onChangeHandlers, cc.onChangeHandlers)

	cc.mu.Unlock()

	if len(onChangeHandlers) > 0 {
		// Recover so the callback cannot break Runtime.
		defer func() {
			if rec := recover(); rec != nil {
				logs.Errorf("on container change hook failed: %v", rec)
			}
		}()

		for _, h := range onChangeHandlers {
			h()
		}
	}
}

func (cc *ContainerConfig) SetPort(port int) {
	cc.mu.Lock()

	cc.port = port
	onChangeHandlers := make([]containerConfigOnChangeHandler, len(cc.onChangeHandlers))
	copy(onChangeHandlers, cc.onChangeHandlers)

	cc.mu.Unlock()

	if len(onChangeHandlers) > 0 {
		// Recover so the callback cannot break Runtime.
		defer func() {
			if rec := recover(); rec != nil {
				logs.Errorf("on container change hook failed: %v", rec)
			}
		}()

		for _, h := range onChangeHandlers {
			h()
		}
	}
}

func NewContainerConfig() *ContainerConfig {
	return &ContainerConfig{
		onChangeHandlers: make([]containerConfigOnChangeHandler, 0, 1),
	}
}
