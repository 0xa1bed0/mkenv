package dockerfile

import (
	"errors"
	"unicode"

	"github.com/0xa1bed0/mkenv/internal/filesmanager"
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

type langDetector struct {
	targetFile     string
	fileExtentions string // coma separated (e.g. ts,js,jsx)
	versionPrefix  string
}

func (ld *langDetector) ScanFiles(folderPtr filesmanager.FileManager) (found bool, brickMeta map[string]string, err error) {
	targetFile := ld.targetFile
	fileExtentions := ld.fileExtentions
	if targetFile == "" || fileExtentions == "" {
		return false, nil, errors.New("at least one targetFile or fileExtentions has to be provided")
	}
	versionPrefix := ld.versionPrefix

	// TODO: maybe we should have global set of folders we ignore?
	ignorePath := []string{"vendor", "node_modules"}

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
	if len(result) == 0 {
		return found, nil, nil
	}

	isVersionChar := func(b byte) bool {
		return unicode.IsDigit(rune(b)) || b == '.' || b == '>' || b == '<' || b == '=' || b == '^' || b == '|'
	}

	versionsFound := []string{}

	for _, gomod := range result {
		scanner, getFileScannerError := folderPtr.GetFileScanner(gomod, 32)
		if getFileScannerError != nil {
			return false, nil, getFileScannerError
		}

		if findError := scanner.Find([]byte(versionPrefix)); findError != nil {
			return false, nil, findError
		}

		version, defineVersionError := scanner.ReadWhile(32, isVersionChar)
		if defineVersionError != nil {
			return false, nil, defineVersionError
		}

		versionsFound = append(versionsFound, string(version))
	}

	brickMeta = make(map[string]string)

	if len(versionsFound) > 0 {
		goVersionToInstall, err := versions.MaxVersionFromConstraints(versionsFound)
		if err != nil {
			if !errors.Is(err, versions.ErrConflictingConstraints) {
				return false, nil, err
			}
			// TODO: print warning that found versions are conflicting
		}
		brickMeta["version"] = goVersionToInstall
	} else {
		// TODO: throw warning that no version found and set lts
	}

	return true, brickMeta, nil
}

func NewLangDetector(targetFile, fileExtentions, versionPrefix string) LangDetector {
	return &langDetector{
		targetFile:     targetFile,
		fileExtentions: fileExtentions,
		versionPrefix:  versionPrefix,
	}
}
