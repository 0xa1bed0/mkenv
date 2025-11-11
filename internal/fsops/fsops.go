// Package fsops exposes thin interfaces over os and filepath helpers so the
// rest of the project can be tested without touching the real filesystem.
package fsops

import (
	"io/fs"
	"os"
	"path/filepath"
)

// PathOps abstracts common filepath operations to allow mocking in tests.
type PathOps interface {
	Abs(path string) (string, error)
	Rel(basepath, targpath string) (string, error)
	Join(elem ...string) string
	Clean(path string) string
	IsAbs(path string) bool
	Ext(name string) string
}

// OSOps abstracts filesystem metadata queries such as os.Stat.
type OSOps interface {
	Stat(name string) (fs.FileInfo, error)
}

// DirWalker abstracts directory walking (e.g., filepath.WalkDir).
type DirWalker interface {
	WalkDir(root string, fn fs.WalkDirFunc) error
}

// Ops groups together the dependencies required by FileManager.
type Ops struct {
	Path   PathOps
	OS     OSOps
	Walker DirWalker
}

// DefaultOps returns an Ops configured with the standard library implementations.
func DefaultOps() Ops {
	return Ops{
		Path:   stdPathOps{},
		OS:     stdOSOps{},
		Walker: stdDirWalker{},
	}
}

type stdPathOps struct{}

func (stdPathOps) Abs(path string) (string, error) { return filepath.Abs(path) }
func (stdPathOps) Rel(basepath, targpath string) (string, error) {
	return filepath.Rel(basepath, targpath)
}
func (stdPathOps) Join(elem ...string) string { return filepath.Join(elem...) }
func (stdPathOps) Clean(path string) string   { return filepath.Clean(path) }
func (stdPathOps) IsAbs(path string) bool     { return filepath.IsAbs(path) }
func (stdPathOps) Ext(name string) string     { return filepath.Ext(name) }

type stdOSOps struct{}

func (stdOSOps) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }

type stdDirWalker struct{}

func (stdDirWalker) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn)
}
