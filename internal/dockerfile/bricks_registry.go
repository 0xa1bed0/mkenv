package dockerfile

import (
	"sync"
)

type (
	BrickFactory    func(metadata map[string]string) (Brick, error)
	DetectorFactory func() BrickDetector
)

type BricksRegistry struct {
	mu        sync.RWMutex
	bricks    map[BrickID]BrickFactory 
	detectors []DetectorFactory       
}

func NewRegistry() *BricksRegistry {
	return &BricksRegistry{
		bricks:    map[BrickID]BrickFactory{},
		detectors: []DetectorFactory{},
	}
}

var DefaultBricksRegistry = NewRegistry()

func RegisterBrick(id BrickID, f BrickFactory) {
	DefaultBricksRegistry.mu.Lock()
	DefaultBricksRegistry.bricks[id] = f
	DefaultBricksRegistry.mu.Unlock()
}

func RegisterDetector(f DetectorFactory) {
	DefaultBricksRegistry.mu.Lock()
	DefaultBricksRegistry.detectors = append(DefaultBricksRegistry.detectors, f)
	DefaultBricksRegistry.mu.Unlock()
}

func (r *BricksRegistry) GetBrickFactory(id BrickID) (BrickFactory, bool) {
	r.mu.RLock()
	f, ok := r.bricks[id]
	r.mu.RUnlock()
	return f, ok
}

func (r *BricksRegistry) ListBrickIDs() []BrickID {
	r.mu.RLock()
	out := make([]BrickID, 0, len(r.bricks))
	for id := range r.bricks {
		out = append(out, id)
	}
	// TODO: fix []BrickID sort
	// sort.Strings(out)
	r.mu.RUnlock()
	return out
}

func (r *BricksRegistry) AllDetectors() []BrickDetector {
	r.mu.RLock()
	fs := append([]DetectorFactory(nil), r.detectors...)
	out := make([]BrickDetector, 0, len(fs))
	for _, mk := range fs {
		out = append(out, mk())
	}
	r.mu.RUnlock()
	return out
}
