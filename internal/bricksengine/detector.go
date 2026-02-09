package bricksengine

import (
	"errors"
	"unicode"

	"github.com/0xa1bed0/mkenv/internal/filesmanager"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/versions"
)

type BrickDetector interface {
	// returns information about which brick this detector will try to detect
	BrickInfo() *BrickInfo
	Scan(folderPtr filesmanager.FileManager) (brickID BrickID, brickMeta map[string]string, err error)
}

type LangDetector interface {
	ScanFiles(folderPtr filesmanager.FileManager) (found bool, brickMeta map[string]string, err error)
}

// VersionSemantics controls how detected version strings are interpreted.
type VersionSemantics string

const (
	// VersionSemanticsConstraint treats versions as exact constraints (default).
	VersionSemanticsConstraint VersionSemantics = "constraint"
	// VersionSemanticsMinimum treats versions as minimum requirements (prepends >=).
	VersionSemanticsMinimum VersionSemantics = "minimum"
)

// LangDetectorOption configures a langDetector.
type LangDetectorOption func(*langDetector)

// WithVersionSemantics sets how detected version strings are interpreted.
func WithVersionSemantics(s VersionSemantics) LangDetectorOption {
	return func(ld *langDetector) {
		ld.versionSemantics = s
	}
}

type langDetector struct {
	brickName        string
	targetFile       string
	fileExtentions   string // coma separated (e.g. ts,js,jsx)
	versionPrefix    string
	versionSemantics VersionSemantics
}

func (ld *langDetector) ScanFiles(folderPtr filesmanager.FileManager) (found bool, brickMeta map[string]string, err error) {
	targetFile := ld.targetFile
	fileExtentions := ld.fileExtentions
	if targetFile == "" || fileExtentions == "" {
		return false, nil, errors.New("at least one targetFile or fileExtentions has to be provided")
	}
	versionPrefix := ld.versionPrefix

	// TODO: maybe we should have global set of folders we ignore?
	ignorePath := []string{"vendor", "node_modules", ".gomod"}

	if fileExtentions != "" {
		hasFiles, er := folderPtr.HasFilesWithExtensions(fileExtentions, ignorePath)
		if er != nil {
			return false, nil, er
		}
		if hasFiles && targetFile == "" {
			return true, nil, nil
		}
		if hasFiles {
			found = true
		}
	}

	result, err := folderPtr.FindFile(targetFile, ignorePath)
	if err != nil {
		return false, brickMeta, err
	}
	logs.Debugf("detector[%s]: found %d %s files: %v", ld.brickName, len(result), targetFile, result)
	if len(result) == 0 {
		return found, nil, nil
	}

	isVersionChar := func(b byte) bool {
		return unicode.IsDigit(rune(b)) || b == '.' || b == '>' || b == '<' || b == '=' || b == '^' || b == '|'
	}

	versionsFound := []string{}

	for _, gomod := range result {
		logs.Debugf("detector[%s]: scanning file %s for prefix %q", ld.brickName, gomod, versionPrefix)
		scanner, getFileScannerError := folderPtr.GetFileScanner(gomod, 32)
		if getFileScannerError != nil {
			return false, nil, getFileScannerError
		}

		if findError := scanner.Find([]byte(versionPrefix)); findError != nil {
			// TODO: make custom error
			if findError.Error() == "prefix not found" {
				logs.Debugf("detector[%s]: prefix %q not found in %s, checking next file", ld.brickName, versionPrefix, gomod)
				continue // Check other files
			}
			return false, nil, findError
		}

		version, defineVersionError := scanner.ReadWhile(32, isVersionChar)
		if defineVersionError != nil {
			return false, nil, defineVersionError
		}

		v := string(version)
		if ld.versionSemantics == VersionSemanticsMinimum {
			v = ">=" + v
		}
		logs.Debugf("detector[%s]: found version %q in %s", ld.brickName, v, gomod)
		versionsFound = append(versionsFound, v)
	}

	brickMeta = make(map[string]string)

	if len(versionsFound) > 0 {
		logs.Debugf("detector[%s]: all versions found: %v", ld.brickName, versionsFound)
		versionToInstall, err := versions.MaxVersionFromConstraints(versionsFound)
		if err != nil {
			if !errors.Is(err, versions.ErrConflictingConstraints) {
				return false, nil, err
			}
			logs.Warnf("Found evidence of conflicting versions for %s. Using the biggest one: %s", ld.brickName, versionToInstall)
		}
		logs.Debugf("detector[%s]: selected version %s", ld.brickName, versionToInstall)
		brickMeta["version"] = versionToInstall
	} else {
		logs.Warnf("Can't estimate version for %s. Using known latest", ld.brickName)
	}

	return true, brickMeta, nil
}

func NewLangDetector(brickName, targetFile, fileExtentions, versionPrefix string, opts ...LangDetectorOption) LangDetector {
	ld := &langDetector{
		brickName:        brickName,
		targetFile:       targetFile,
		fileExtentions:   fileExtentions,
		versionPrefix:    versionPrefix,
		versionSemantics: VersionSemanticsConstraint,
	}
	for _, opt := range opts {
		opt(ld)
	}
	return ld
}
