// Package languages contains container bricks that configure programming
// language toolchains.
package languages

import (
	"errors"
	"unicode"

	"github.com/0xa1bed0/mkenv/internal/dockerimage"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
	"github.com/0xa1bed0/mkenv/internal/versions"
)

// Lang describes a language-specific devcontainer brick capable of detecting
// language usage within a project.
type Lang interface {
	dockerimage.Brick
	// TestEnvironment checks whether the project folder contains source files
	// for the language.
	TestEnvironment(fileManager filesmanager.FileManager) (bool, error)
	// SetVersion sets the language toolchain version manually.
	SetVersion(version string)
	GetVersion() string
}

type LangBrickBase struct {
	dockerimage.BrickBase
}

func (lang *LangBrickBase) SetVersion(version string) {
	lang.SetParam("version", version)
}

func (lang *LangBrickBase) GetVersion() string {
	version, _ := lang.GetParam("version")
	return version
}

func (lang *LangBrickBase) TestEnvironment(fileManager filesmanager.FileManager) (bool, error) {
	targetFile, ok := lang.GetParam("targetFile")
	if !ok {
		return false, errors.New("language misconfiguration: targetFile")
	}

	versionPrefix, ok := lang.GetParam("versionPrefix")
	if !ok {
		return false, errors.New("language misconfiguration: versionPrefix")
	}

	// TODO: maybe we should have global set of folders we ignore?
	result, err := fileManager.FindFile(targetFile, []string{"vendor", "node_modules"})
	if err != nil {
		return false, err
	}
	if len(result) == 0 {
		return false, nil
	}

	isVersionChar := func(b byte) bool {
		return unicode.IsDigit(rune(b)) || b == '.' || b == '>' || b == '<' || b == '=' || b == '^' || b == '|'
	}

	versionsFound := []string{}

	for _, gomod := range result {
		scanner, getFileScannerError := fileManager.GetFileScanner(gomod, 32)
		if getFileScannerError != nil {
			return false, getFileScannerError
		}

		if findError := scanner.Find([]byte(versionPrefix)); findError != nil {
			return false, findError
		}

		version, defineVersionError := scanner.ReadWhile(32, isVersionChar)
		if defineVersionError != nil {
			return false, defineVersionError
		}

		versionsFound = append(versionsFound, string(version))
	}

	if len(versionsFound) > 0 {
		goVersionToInstall, err := versions.MaxVersionFromConstraints(versionsFound)
		if err != nil {
			if !errors.Is(err, versions.ErrConflictingConstraints) {
				return false, err
			}
			// TODO: print warning that found versions are conflicting
		}
		lang.SetVersion(goVersionToInstall)
	} else {
		// TODO: throw warning that no version found and set lts
	}

	return true, nil
}

// GetEnabledLangBricks returns the bricks that should be enabled for a project
// based on language auto-detection.
func GetEnabledLangBricks(projectFolderPtr filesmanager.FileManager) ([]dockerimage.Brick, error) {
	langs := []Lang{
		NewLangGolang(),
		NewLangNodejs(),
	}

	enabledLangs := []dockerimage.Brick{}

	for _, lang := range langs {
		ok, err := lang.TestEnvironment(projectFolderPtr)
		if err != nil {
			return nil, err
		}

		if !ok {
			continue
		}

		enabledLangs = append(enabledLangs, lang)
	}

	return enabledLangs, nil
}
