package hostappconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

// ensureFolder recursively creates a folder if it does not exist.
func ensureFolder(path string) error {
	return os.MkdirAll(path, 0o755)
}

// ensureFile ensures that the parent folder exists and the file exists.
// If the file already exists, it does nothing.
func ensureFile(path string) error {
	// Ensure parent directory
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Create file if not exists
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create/open file: %w", err)
	}
	defer f.Close()

	return nil
}

func ConfigBasePath() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		// TODO: are we sure??? what about non unix systems? do we really want to support them?
		homedir = "/usr/local/config/mkenv"
	}

	p := filepath.Join(homedir, ".config", "mkenv")
	return p
}

func projectDataPath(projectName string) string {
	p := filepath.Join(ConfigBasePath(), "projects", projectName)
	return p
}

func logsPath(projectName string) string {
	p := filepath.Join(projectDataPath(projectName), "logs")
	return p
}

func StateDBFile() string {
	return filepath.Join(ConfigBasePath(), "state.db")
}

func RunLogPath(projectName, runID string) string {
	p := filepath.Join(logsPath(projectName), "run-"+runID+".log")
	ensureFile(p)
	return p
}

func RunLogPathOpen(projectName, runID string) (*os.File, error) {
	return os.OpenFile(RunLogPath(projectName, runID), os.O_CREATE|os.O_RDWR, 0o644)
}

func AgentLogsPathOnHost(projectName, runID string) string {
	p := filepath.Join(logsPath(projectName), "agent-run-"+runID+".log")
	ensureFile(p)
	return p
}

func AgentBinaryPath(projectName string) string {
	p := filepath.Join(projectDataPath(projectName), "bin")
	ensureFolder(p)
	return p
}

func ContainerProxyPort() int {
	return 45454
}
