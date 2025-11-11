// Package filesmanager provides high-level helpers for discovering project files
// and streaming their contents.
package filesmanager

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/fsops"
)

// FileManager wraps file discovery and streaming readers rooted at a project
// directory.
type FileManager interface {
	// FindFile finds all files with given filenames in the directory.
	// Provide ignorePaths to skip specific subdirectories.
	FindFile(filename string, ignorePaths []string) ([]string, error)

	// HasFilesWithExtensions reports whether there is at least one file whose
	// extension is in extsCSV (comma-separated, items like "go, ts, .tsx").
	// It respects ignorePaths the same way FindFile does.
	HasFilesWithExtensions(extsCSV string, ignorePaths []string) (bool, error)

	// GetFileScanner provides FileScanner for an existing file in the directory.
	GetFileScanner(filepath string, bufSize uint8) (FileScanner, error)
}

type folderPtr struct {
	root string
	ops  fsops.Ops
}

// GetFileScanner implements FileManager.
func (ptr *folderPtr) GetFileScanner(filepath string, bufSize uint8) (FileScanner, error) {
	// TODO: check if file is in root and check if there is no ..
	return newFileScanner(ptr.root+"/"+filepath, bufSize)
}

// NewFileManager builds a FileManager rooted at dir using the default OS
// implementations.
func NewFileManager(dir string) (FileManager, error) {
	return NewFileManagerWithOps(dir, fsops.DefaultOps())
}

// NewFileManagerWithOps is the internal constructor that allows injecting
// filesystem dependencies for testing.
func NewFileManagerWithOps(dir string, ops fsops.Ops) (FileManager, error) {
	if dir == "" {
		return nil, errors.New("folder path should not be empty")
	}

	if ops.Path == nil || ops.OS == nil || ops.Walker == nil {
		return nil, errors.New("file manager dependencies cannot be nil")
	}

	abs, err := ops.Path.Abs(dir)
	if err != nil {
		return nil, err
	}

	fi, err := ops.OS.Stat(abs)
	if err != nil {
		return nil, err
	}

	if !fi.IsDir() {
		return nil, errors.New("root path is not a directory")
	}

	return &folderPtr{
		root: ops.Path.Clean(abs),
		ops:  ops,
	}, nil
}

func (p *folderPtr) HasFilesWithExtensions(extsCSV string, ignorePaths []string) (bool, error) {
	exts, err := parseExtsCSV(extsCSV)
	if err != nil {
		return false, err
	}
	if len(exts) == 0 {
		return false, nil
	}

	// Normalize ignore list (same logic as in FindFile)
	ignoreAbs := make([]string, 0, len(ignorePaths))
	for _, q := range ignorePaths {
		if q == "" {
			continue
		}
		var abs string
		if p.ops.Path.IsAbs(q) {
			rel, err := p.ops.Path.Rel(p.root, q)
			if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
				continue // outside root
			}
			abs = p.ops.Path.Join(p.root, rel)
		} else {
			abs = p.ops.Path.Join(p.root, q)
		}
		ignoreAbs = append(ignoreAbs, p.ops.Path.Clean(abs))
	}
	shouldSkip := func(absPath string, isDir bool) (skipNode, skipTree bool) {
		for _, ig := range ignoreAbs {
			if samePath(absPath, ig) {
				if isDir {
					return true, true
				}
				return true, false
			}
		}
		return false, false
	}

	found := false
	sentinel := errors.New("found") // used to break WalkDir early

	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		abs := p.ops.Path.Clean(path)

		if skipNode, skipTree := shouldSkip(abs, d.IsDir()); skipNode {
			if skipTree {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(p.ops.Path.Ext(d.Name()))
		if _, ok := exts[ext]; ok {
			found = true
			return sentinel // stop walking early
		}
		return nil
	}

	if err := p.ops.Walker.WalkDir(p.root, walkFn); err != nil && !errors.Is(err, sentinel) {
		return false, err
	}
	return found, nil
}

// parseExtsCSV turns "go, ts, .tsx" into map{".go":{}, ".ts":{}, ".tsx":{}}
func parseExtsCSV(csv string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	for _, raw := range strings.Split(csv, ",") {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, ".") {
			s = "." + s
		}
		s = strings.ToLower(s)
		out[s] = struct{}{}
	}
	return out, nil
}

func (ptr *folderPtr) FindFile(filename string, ignorePaths []string) ([]string, error) {
	if filename == "" {
		return nil, errors.New("filename is empty")
	}

	if !isPlainFilename(filename) {
		return nil, fmt.Errorf("%s is not a filename", filename)
	}

	// Normalize ignore list to absolute-cleaned paths under fm.root.
	// We treat entries as relative to root; absolute entries outside root are ignored.
	ignoreAbs := make([]string, 0, len(ignorePaths))
	for _, p := range ignorePaths {
		if p == "" {
			continue
		}
		var abs string
		if ptr.ops.Path.IsAbs(p) {
			// Only accept if it’s inside the root.
			rel, err := ptr.ops.Path.Rel(ptr.root, p)
			if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
				// Path outside root – ignore silently.
				continue
			}
			abs = ptr.ops.Path.Join(ptr.root, rel)
		} else {
			abs = ptr.ops.Path.Join(ptr.root, p)
		}
		ignoreAbs = append(ignoreAbs, ptr.ops.Path.Clean(abs))
	}

	// Helper: should skip this path or its subtree?
	shouldSkip := func(absPath string, isDir bool) (skipThis bool, skipDir bool) {
		// Exact match: skip the node; if it's a dir, skip its subtree.
		for _, ig := range ignoreAbs {
			if samePath(absPath, ig) {
				if isDir {
					return true, true // skip dir subtree
				}
				return true, false // skip file
			}
		}
		return false, false
	}

	var results []string

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Propagate filesystem errors (permissions, etc.)
			return err
		}
		abs := ptr.ops.Path.Clean(path)

		// Skip ignored nodes (and subtrees if a directory).
		if skipNode, skipTree := shouldSkip(abs, d.IsDir()); skipNode {
			if skipTree {
				return fs.SkipDir
			}
			return nil
		}

		// We only match files (not directories) by base name.
		if !d.IsDir() && d.Name() == filename {
			rel, err := ptr.ops.Path.Rel(ptr.root, abs)
			if err != nil {
				return err
			}
			// Normalize to forward slashes for stable output (cross-platform).
			results = append(results, toSlashClean(rel))
		}
		return nil
	}

	if err := ptr.ops.Walker.WalkDir(ptr.root, walkFn); err != nil {
		return nil, err
	}

	// Stable ordering helps with tests and determinism.
	sort.Strings(results)
	return results, nil
}
