package langs

import (
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/filesmanager"
)

const (
	phpID          = bricksengine.BrickID("php")
	phpDescription = "PHP toolchain"
)

var phpKinds = []bricksengine.BrickKind{bricksengine.BrickKindCommon}

func NewPHP(metadata map[string]string) (bricksengine.Brick, error) {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	version, ok := metadata["version"]
	if !ok || version == "" {
		version = ""
	}

	brick, err := bricksengine.NewBrick(phpID, phpDescription,
		bricksengine.WithKinds(phpKinds),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "php install dependencies",
			Packages: []bricksengine.PackageSpec{
				{Name: "curl"},
				{Name: "ca-certificates"},
				{Name: "git"},
				{Name: "php" + version},
			},
		}),
	)
	if err != nil {
		return nil, err
	}

	return brick, nil
}

type phpDetector struct {
	langDetector bricksengine.LangDetector
}

func (*phpDetector) BrickInfo() *bricksengine.BrickInfo {
	return bricksengine.NewBrickInfo(phpID, phpDescription, phpKinds)
}

func (gd *phpDetector) Scan(folderPtr filesmanager.FileManager) (bricksengine.BrickID, map[string]string, error) {
	found, brickMeta, err := gd.langDetector.ScanFiles(folderPtr)
	if err != nil {
		return "", nil, err
	}
	if found {
		return phpID, brickMeta, nil
	}
	return "", nil, nil
}

func init() {
	bricksengine.RegisterBrick(phpID, NewPHP)
	bricksengine.RegisterDetector(func() bricksengine.BrickDetector {
		return &phpDetector{langDetector: bricksengine.NewLangDetector(string(phpID), "composer.json", "php", `"php": "`)}
	})
}
