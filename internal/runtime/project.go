package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/filesmanager"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/state"
	"github.com/0xa1bed0/mkenv/internal/utils"
)

type projectStateDB struct {
	kvStore *state.KVStore
}

func newProjectStateDB(db *state.KVStore) *projectStateDB {
	return &projectStateDB{kvStore: db}
}

func (st *projectStateDB) deriveKey(path string) state.KVStoreKey {
	return state.KVStoreKey("project:" + path)
}

func (st *projectStateDB) isKnownPath(ctx context.Context, path string) bool {
	if st.kvStore == nil {
		logs.Warnf("[projectState:isKnownPath] can't determine if this project known because no state provided. Assuming is not know")
		return false
	}

	key := st.deriveKey(path)

	_, found, err := st.kvStore.Get(ctx, key)
	if err != nil {
		logs.Warnf("[projectState:isKnownPath] can't determine if this project known. Assuming is not know. error: %v", err)
		return false
	}

	return found
}

func (st *projectStateDB) setKnown(ctx context.Context, path string) {
	if st.kvStore == nil {
		logs.Warnf("[projectState:isKnownPath] can't set project known because no database provided. Skipping")
		return
	}

	key := st.deriveKey(path)

	err := st.kvStore.Upsert(ctx, key, "known")
	if err != nil {
		logs.Warnf("[projectState:isKnownPath] can't set project known. Skipping... \nerror: %v", err)
	}
}

type Project struct {
	name  string
	path  string
	known bool

	envConfig         EnvConfig
	envConfigOverride EnvConfig

	folderPtr filesmanager.FileManager
	stateDB   *projectStateDB
}

func (p *Project) Signature(ctx context.Context) (string, error) {
	// TODO: remove ctx from here. Maybe even move this function back to state (this needed for key derivation)
	// TODO: maybe just add String() function to env config
	envConfigSignature, err := p.EnvConfig(ctx).Signature()
	if err != nil {
		return "", err
	}
	type sigpayload struct {
		Path               string `json:"project_path"`
		EnvConfigSignature string `json:"env_config_sig"`
	}

	payload := &sigpayload{
		Path:               p.Path(),
		EnvConfigSignature: envConfigSignature,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func resolveProject(ctx context.Context, p string, stateDB *projectStateDB) (*Project, error) {
	path, err := utils.ResolveFolderStrict(p)
	if err != nil {
		return nil, err
	}

	known := false
	if stateDB != nil {
		known = stateDB.isKnownPath(ctx, path)
	}

	name := resolveProjectName(path)

	project := &Project{
		name:    name,
		path:    path,
		known:   known,
		stateDB: stateDB,
	}

	return project, nil
}

func (p *Project) Path() string {
	return p.path
}

func (p *Project) Name() string {
	return p.name
}

func (p *Project) Known() bool {
	return p.known
}

func (p *Project) EnvConfig(ctx context.Context) EnvConfig {
	if p.envConfig == nil {
		err := p.resolveEnvConfig(ctx)
		if err != nil {
			panic(err)
		}
	}

	return p.envConfig
}

func (p *Project) FolderPtr() (filesmanager.FileManager, error) {
	if p.folderPtr == nil {
		var err error
		p.folderPtr, err = filesmanager.NewFileManager(p.path)
		if err != nil {
			return nil, err
		}
	}

	return p.folderPtr, nil
}

func (p *Project) SetKnown(ctx context.Context) {
	if p.stateDB == nil {
		logs.Warnf("[Project:SetKnown] project state DB is not initialized. Skipping project state mutation...")
		return
	}

	p.stateDB.setKnown(ctx, p.path)
	p.known = true
}

// ------------- utils -------------

var invalidNameChars = regexp.MustCompile(`[^a-z0-9._-]+`)

// resolveProjectName: encodes (almost) the full path into a Docker-safe (almost) name.
func resolveProjectName(abs string) string {
	home, _ := os.UserHomeDir()
	asSlash := filepath.ToSlash(abs)
	homeSlash := filepath.ToSlash(home)

	if homeSlash != "" && strings.HasPrefix(asSlash, homeSlash) {
		asSlash = strings.Replace(asSlash, homeSlash, "home", 1)
	}
	asSlash = strings.TrimPrefix(asSlash, "/")

	if runtime.GOOS == "windows" {
		if len(asSlash) >= 2 && asSlash[1] == ':' {
			asSlash = asSlash[2:]
			asSlash = strings.TrimPrefix(asSlash, "/")
		}
	}

	name := strings.ToLower(strings.ReplaceAll(asSlash, "/", "-"))
	name = invalidNameChars.ReplaceAllString(name, "_")
	name = strings.TrimLeft(name, ".-")
	if name == "" {
		name = "anonymous-project"
	}

	// Don't constrain project length here; we handle final length where needed
	return name
}
