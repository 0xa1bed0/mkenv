package filesmanager

import (
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/0xa1bed0/mkenv/internal/fsops"
	fsopsMocks "github.com/0xa1bed0/mkenv/internal/fsops/mocks"
	"go.uber.org/mock/gomock"
)

type fakeFileInfo struct {
	name  string
	isDir bool
}

func (f fakeFileInfo) Name() string      { return f.name }
func (f fakeFileInfo) Size() int64       { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode {
	if f.isDir {
		return fs.ModeDir
	}
	return 0
}
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() any           { return nil }

type fakeDirEntry struct {
	name  string
	isDir bool
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return f.isDir }
func (f fakeDirEntry) Type() fs.FileMode {
	if f.isDir {
		return fs.ModeDir
	}
	return 0
}
func (f fakeDirEntry) Info() (fs.FileInfo, error) { return fakeFileInfo{name: f.name, isDir: f.isDir}, nil }

func TestNewFileManagerWithOps_Validation(t *testing.T) {
	t.Parallel()

	if _, err := NewFileManagerWithOps("", fsops.Ops{}); err == nil {
		t.Fatal("expected error for empty directory")
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pathOps := fsopsMocks.NewMockPathOps(ctrl)
	osOps := fsopsMocks.NewMockOSOps(ctrl)
	walker := fsopsMocks.NewMockDirWalker(ctrl)

	ops := fsops.Ops{Path: nil, OS: osOps, Walker: walker}
	if _, err := NewFileManagerWithOps("root", ops); err == nil {
		t.Fatal("expected error when Path dependency is nil")
	}

	pathOps = fsopsMocks.NewMockPathOps(ctrl)
	ops = fsops.Ops{Path: pathOps, OS: nil, Walker: walker}
	if _, err := NewFileManagerWithOps("root", ops); err == nil {
		t.Fatal("expected error when OS dependency is nil")
	}

	ops = fsops.Ops{Path: pathOps, OS: osOps, Walker: nil}
	if _, err := NewFileManagerWithOps("root", ops); err == nil {
		t.Fatal("expected error when Walker dependency is nil")
	}
}

func TestNewFileManagerWithOps_Success(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pathOps := fsopsMocks.NewMockPathOps(ctrl)
	osOps := fsopsMocks.NewMockOSOps(ctrl)
	walker := fsopsMocks.NewMockDirWalker(ctrl)

	const input = "root"
	const absRoot = "/abs/root"

	gomock.InOrder(
		pathOps.EXPECT().Abs(input).Return(absRoot, nil),
		osOps.EXPECT().Stat(absRoot).Return(fakeFileInfo{name: absRoot, isDir: true}, nil),
		pathOps.EXPECT().Clean(absRoot).Return(absRoot),
	)

	ops := fsops.Ops{Path: pathOps, OS: osOps, Walker: walker}
	fm, err := NewFileManagerWithOps(input, ops)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm == nil {
		t.Fatal("expected non-nil FileManager")
	}
}

func TestNewFileManagerWithOps_ErrorsPropagated(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pathOps := fsopsMocks.NewMockPathOps(ctrl)
	osOps := fsopsMocks.NewMockOSOps(ctrl)
	walker := fsopsMocks.NewMockDirWalker(ctrl)

	absErr := errors.New("abs failure")
	pathOps.EXPECT().Abs("root").Return("", absErr)
	if _, err := NewFileManagerWithOps("root", fsops.Ops{Path: pathOps, OS: osOps, Walker: walker}); !errors.Is(err, absErr) {
		t.Fatalf("expected abs error, got %v", err)
	}

	// Stat error
	pathOps.EXPECT().Abs("root").Return("/abs/root", nil)
	statErr := errors.New("stat failure")
	osOps.EXPECT().Stat("/abs/root").Return(nil, statErr)
	if _, err := NewFileManagerWithOps("root", fsops.Ops{Path: pathOps, OS: osOps, Walker: walker}); !errors.Is(err, statErr) {
		t.Fatalf("expected stat error, got %v", err)
	}

	// Not a directory
	pathOps.EXPECT().Abs("root").Return("/abs/root", nil)
	osOps.EXPECT().Stat("/abs/root").Return(fakeFileInfo{name: "file", isDir: false}, nil)
	if _, err := NewFileManagerWithOps("root", fsops.Ops{Path: pathOps, OS: osOps, Walker: walker}); err == nil || err.Error() != "root path is not a directory" {
		t.Fatalf("expected not-a-directory error, got %v", err)
	}
}

func TestFindFileErrors(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pathOps := fsopsMocks.NewMockPathOps(ctrl)
	osOps := fsopsMocks.NewMockOSOps(ctrl)
	walker := fsopsMocks.NewMockDirWalker(ctrl)

	const absRoot = "/abs/root"

	pathOps.EXPECT().Abs("root").Return(absRoot, nil)
	osOps.EXPECT().Stat(absRoot).Return(fakeFileInfo{name: absRoot, isDir: true}, nil)

	pathOps.EXPECT().Clean(gomock.Any()).DoAndReturn(func(p string) string {
		return filepath.Clean(p)
	}).AnyTimes()
	pathOps.EXPECT().IsAbs(gomock.Any()).Return(false).AnyTimes()

	fmIface, err := NewFileManagerWithOps("root", fsops.Ops{Path: pathOps, OS: osOps, Walker: walker})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if _, err := fmIface.FindFile("", nil); err == nil {
		t.Fatal("expected error for empty filename")
	}

	if _, err := fmIface.FindFile("nested/file", nil); err == nil {
		t.Fatal("expected error for non plain filename")
	}
}

func TestFindFileRespectsIgnoreAndSorts(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pathOps := fsopsMocks.NewMockPathOps(ctrl)
	osOps := fsopsMocks.NewMockOSOps(ctrl)
	walker := fsopsMocks.NewMockDirWalker(ctrl)

	const rootInput = "root"
	const absRoot = "/abs/root"

	pathOps.EXPECT().Abs(rootInput).Return(absRoot, nil)
	osOps.EXPECT().Stat(absRoot).Return(fakeFileInfo{name: absRoot, isDir: true}, nil)
	pathOps.EXPECT().Clean(absRoot).Return(absRoot)

	pathOps.EXPECT().Clean(gomock.Any()).DoAndReturn(func(p string) string {
		return filepath.Clean(p)
	}).AnyTimes()
	pathOps.EXPECT().IsAbs(gomock.Any()).DoAndReturn(func(p string) bool {
		return filepath.IsAbs(p)
	}).AnyTimes()
	pathOps.EXPECT().Join(gomock.Any(), gomock.Any()).DoAndReturn(func(parts ...string) string {
		return filepath.Join(parts...)
	}).AnyTimes()
	pathOps.EXPECT().Rel(gomock.Any(), gomock.Any()).DoAndReturn(func(a, b string) (string, error) {
		return filepath.Rel(a, b)
	}).AnyTimes()

	// Walk order is intentionally unsorted to verify sorting behavior.
	walker.EXPECT().WalkDir(absRoot, gomock.Any()).DoAndReturn(func(root string, fn fs.WalkDirFunc) error {
		type entry struct {
			path string
			dir  bool
		}
		entries := []entry{
			{path: absRoot, dir: true},
			{path: filepath.Join(absRoot, "zeta"), dir: true},
			{path: filepath.Join(absRoot, "zeta", "go.mod"), dir: false},
			{path: filepath.Join(absRoot, "alpha"), dir: true},
			{path: filepath.Join(absRoot, "alpha", "go.mod"), dir: false},
			{path: filepath.Join(absRoot, "beta"), dir: true},
			{path: filepath.Join(absRoot, "beta", "go.mod"), dir: false},
			{path: filepath.Join(absRoot, "skip"), dir: true},
			{path: filepath.Join(absRoot, "skip", "go.mod"), dir: false},
			{path: filepath.Join(absRoot, "skipabs"), dir: true},
			{path: filepath.Join(absRoot, "skipabs", "go.mod"), dir: false},
		}

		skipped := map[string]struct{}{}

		for _, e := range entries {
			skip := false
			for prefix := range skipped {
				if strings.HasPrefix(e.path, prefix) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			entry := fakeDirEntry{name: filepath.Base(e.path), isDir: e.dir}

			err := fn(e.path, entry, nil)
			if err == fs.SkipDir {
				skipped[e.path+string(filepath.Separator)] = struct{}{}
				continue
			}
			if err != nil {
				return err
			}
		}
		return nil
	})

	fmIface, err := NewFileManagerWithOps(rootInput, fsops.Ops{Path: pathOps, OS: osOps, Walker: walker})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	got, err := fmIface.FindFile("go.mod", []string{
		"skip",
		"beta/go.mod",
		filepath.Join(absRoot, "skipabs"),
		filepath.Join(absRoot, "..", "outside"),
		"",
	})
	if err != nil {
		t.Fatalf("FindFile failed: %v", err)
	}

	want := []string{
		"alpha/go.mod",
		"zeta/go.mod",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FindFile returned %v, want %v", got, want)
	}
}
