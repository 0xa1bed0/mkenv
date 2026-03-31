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
	phpVersionDetector bricksengine.LangDetector // .php-version (priority)
	langDetector       bricksengine.LangDetector // composer.json (fallback)
}

func (*phpDetector) BrickInfo() *bricksengine.BrickInfo {
	return bricksengine.NewBrickInfo(phpID, phpDescription, phpKinds)
}

func (pd *phpDetector) Scan(folderPtr filesmanager.FileManager) (bricksengine.BrickID, map[string]string, error) {
	// Priority: check .php-version first
	pvFound, pvMeta, err := pd.phpVersionDetector.ScanFiles(folderPtr)
	if err != nil {
		return "", nil, err
	}

	pvVersion := ""
	if pvMeta != nil {
		pvVersion = pvMeta["version"]
	}

	// If .php-version has a version, use it
	if pvVersion != "" {
		return phpID, pvMeta, nil
	}

	// Fallback: check composer.json
	fallbackFound, fallbackMeta, err := pd.langDetector.ScanFiles(folderPtr)
	if err != nil {
		return "", nil, err
	}

	if !pvFound && !fallbackFound {
		return "", nil, nil
	}

	return phpID, fallbackMeta, nil
}

func init() {
	bricksengine.RegisterBrick(phpID, NewPHP)
	bricksengine.RegisterDetector(func() bricksengine.BrickDetector {
		return &phpDetector{
			phpVersionDetector: bricksengine.NewLangDetector(
				string(phpID), ".php-version", "php", "",
				bricksengine.WithVersionSemantics(bricksengine.VersionSemanticsMinimum),
			),
			langDetector: bricksengine.NewLangDetector(string(phpID), "composer.json", "php", `"php": "`),
		}
	})
}
