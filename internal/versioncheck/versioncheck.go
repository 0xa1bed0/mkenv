// Package versioncheck provides functionality to check for new versions of mkenv.
package versioncheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/0xa1bed0/mkenv/internal/state"
	"github.com/0xa1bed0/mkenv/internal/version"
	"github.com/0xa1bed0/mkenv/internal/versions"
)

const (
	// GitHubOwner is the GitHub repository owner.
	GitHubOwner = "0xa1bed0"
	// GitHubRepo is the GitHub repository name.
	GitHubRepo = "mkenv"

	// CacheTTL is how long to cache the version check result.
	CacheTTL = 24 * time.Hour
	// RequestTimeout is the timeout for the GitHub API request.
	RequestTimeout = 5 * time.Second

	// Cache keys for KVStore
	cacheKeyStable = state.KVStoreKey("versioncheck:stable")
	cacheKeyDev    = state.KVStoreKey("versioncheck:dev")
)

// InstallMethod represents how mkenv was installed.
type InstallMethod int

const (
	InstallMethodUnknown InstallMethod = iota
	InstallMethodHomebrew
	InstallMethodDownload      // Direct binary download
	InstallMethodCompiledBuild // User compiled from source (compiled or compiled-<commit>)
	InstallMethodDevBuild      // CI dev build (dev-<commit>)
)

// devVersionRegex matches dev-<commit> format (CI builds).
var devVersionRegex = regexp.MustCompile(`^dev-([a-f0-9]+)$`)

// compiledVersionRegex matches compiled-<commit> format (user source builds with git).
var compiledVersionRegex = regexp.MustCompile(`^compiled-([a-f0-9]+)$`)

// semverRegex matches semantic version format (optionally prefixed with v).
var semverRegex = regexp.MustCompile(`^v?(\d+\.\d+\.\d+.*)$`)

// githubRelease represents the GitHub API response for a release.
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// githubCommit represents the GitHub API response for a commit.
type githubCommit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
}

// cacheData represents cached version check data.
type cacheData struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// Result contains the version check result.
type Result struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateURL       string
	UpdateAvailable bool
	InstallMethod   InstallMethod
}

// Check checks for a new version of mkenv.
// Returns nil if the current version is "local" (dev build) or if the check fails silently.
func Check(ctx context.Context) *Result {
	current := version.Get()

	// Skip check for local dev builds
	if current == "local" {
		return nil
	}

	// Skip check for compiled builds without commit (zip download without git)
	if current == "compiled" {
		return nil
	}

	// Compiled build from source with git (compiled-<commit>)
	if match := compiledVersionRegex.FindStringSubmatch(current); match != nil {
		return checkCommitVersion(ctx, current, match[1], InstallMethodCompiledBuild)
	}

	// CI dev build (dev-<commit>)
	if match := devVersionRegex.FindStringSubmatch(current); match != nil {
		return checkCommitVersion(ctx, current, match[1], InstallMethodDevBuild)
	}

	// Stable release (v1.2.3)
	if semverRegex.MatchString(current) {
		return checkStableVersion(ctx, current)
	}

	// Unknown version format, skip check
	return nil
}

// checkCommitVersion checks if there's a newer commit on main branch.
// Used for both local-<commit> and dev-<commit> versions.
func checkCommitVersion(ctx context.Context, current, currentCommit string, method InstallMethod) *Result {
	// Try to get from cache first
	cached, cacheAge, err := loadCache(ctx, cacheKeyDev)
	if err == nil && cacheAge < CacheTTL {
		return buildCommitResult(current, currentCommit, cached.Version, cached.URL, method)
	}

	// Fetch latest commit from main branch
	latestCommit, commitURL, err := fetchLatestCommit()
	if err != nil {
		// On error, return cached result if available
		if cached != nil {
			return buildCommitResult(current, currentCommit, cached.Version, cached.URL, method)
		}
		return nil
	}

	// Save to cache
	saveCache(ctx, cacheKeyDev, &cacheData{
		Version: latestCommit,
		URL:     commitURL,
	})

	return buildCommitResult(current, currentCommit, latestCommit, commitURL, method)
}

// checkStableVersion checks if there's a newer stable release.
func checkStableVersion(ctx context.Context, current string) *Result {
	// Try to get from cache first
	cached, cacheAge, err := loadCache(ctx, cacheKeyStable)
	if err == nil && cacheAge < CacheTTL {
		return buildStableResult(current, cached.Version, cached.URL)
	}

	// Fetch latest release from GitHub
	latest, releaseURL, err := fetchLatestRelease()
	if err != nil {
		// On error, return cached result if available
		if cached != nil {
			return buildStableResult(current, cached.Version, cached.URL)
		}
		return nil
	}

	// Save to cache
	saveCache(ctx, cacheKeyStable, &cacheData{
		Version: latest,
		URL:     releaseURL,
	})

	return buildStableResult(current, latest, releaseURL)
}

// buildCommitResult creates a Result for commit-based version comparison.
func buildCommitResult(current, currentCommit, latestCommit, commitURL string, method InstallMethod) *Result {
	// Short commit comparison (first 7 chars)
	latestShort := latestCommit
	if len(latestShort) > 7 {
		latestShort = latestShort[:7]
	}

	// Update is available if commits differ
	updateAvailable := !strings.HasPrefix(latestCommit, currentCommit) &&
		!strings.HasPrefix(currentCommit, latestCommit)

	return &Result{
		CurrentVersion:  current,
		LatestVersion:   latestShort,
		UpdateURL:       commitURL,
		UpdateAvailable: updateAvailable,
		InstallMethod:   method,
	}
}

// buildStableResult creates a Result for stable version comparison.
func buildStableResult(current, latest, releaseURL string) *Result {
	// Normalize versions (remove "v" prefix if present)
	currentNorm := strings.TrimPrefix(current, "v")
	latestNorm := strings.TrimPrefix(latest, "v")

	// Check if update is available using version comparison
	updateAvailable := false
	if versions.IsValidVersion(latestNorm) && versions.IsValidVersion(currentNorm) {
		cmp := versions.Compare(latestNorm, currentNorm)
		updateAvailable = cmp > 0
	}

	// Detect install method based on executable path
	installMethod := detectInstallMethod()

	return &Result{
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateURL:       releaseURL,
		UpdateAvailable: updateAvailable,
		InstallMethod:   installMethod,
	}
}

// fetchLatestRelease fetches the latest stable release from GitHub.
func fetchLatestRelease() (string, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", GitHubOwner, GitHubRepo)

	client := &http.Client{Timeout: RequestTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	return release.TagName, release.HTMLURL, nil
}

// fetchLatestCommit fetches the latest commit on main branch from GitHub.
func fetchLatestCommit() (string, string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/main", GitHubOwner, GitHubRepo)

	client := &http.Client{Timeout: RequestTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch latest commit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var commit githubCommit
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return "", "", fmt.Errorf("failed to decode response: %w", err)
	}

	return commit.SHA, commit.HTMLURL, nil
}

// loadCache loads cached data from KVStore.
// Returns the data, age since last update, and any error.
func loadCache(ctx context.Context, key state.KVStoreKey) (*cacheData, time.Duration, error) {
	kv, err := state.DefaultKVStore(ctx)
	if err != nil {
		return nil, 0, err
	}

	entry, found, err := kv.Get(ctx, key)
	if err != nil {
		return nil, 0, err
	}
	if !found {
		return nil, 0, fmt.Errorf("cache not found")
	}

	var data cacheData
	if err := json.Unmarshal([]byte(entry.Value), &data); err != nil {
		return nil, 0, err
	}

	age := time.Since(entry.LastUsed)
	return &data, age, nil
}

// saveCache saves data to KVStore cache.
func saveCache(ctx context.Context, key state.KVStoreKey, data *cacheData) error {
	kv, err := state.DefaultKVStore(ctx)
	if err != nil {
		return err
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return kv.Upsert(ctx, key, string(jsonData))
}

// detectInstallMethod tries to determine how mkenv was installed based on executable path.
func detectInstallMethod() InstallMethod {
	execPath, err := os.Executable()
	if err != nil {
		return InstallMethodUnknown
	}

	// Resolve symlinks to get the real path
	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		realPath = execPath
	}

	// Check for Homebrew installation
	// Common paths: /opt/homebrew/Cellar/... (Apple Silicon)
	//               /usr/local/Cellar/... (Intel Mac)
	//               /home/linuxbrew/.linuxbrew/Cellar/... (Linux)
	if strings.Contains(realPath, "/Cellar/") ||
		strings.Contains(realPath, "/homebrew/") ||
		strings.Contains(realPath, "/linuxbrew/") {
		return InstallMethodHomebrew
	}

	return InstallMethodDownload
}

// PrintUpdateBanner prints an update notification banner if an update is available.
// This should be called after command execution to avoid interrupting the main flow.
func PrintUpdateBanner(result *Result) {
	if result == nil || !result.UpdateAvailable {
		return
	}

	fmt.Printf("\n")
	fmt.Printf("  A new version of mkenv is available: %s -> %s\n", result.CurrentVersion, result.LatestVersion)

	switch result.InstallMethod {
	case InstallMethodHomebrew:
		fmt.Printf("  Run: brew upgrade mkenv\n")
	case InstallMethodCompiledBuild:
		fmt.Printf("  Pull latest changes and rebuild: git pull && make\n")
	case InstallMethodDevBuild, InstallMethodDownload, InstallMethodUnknown:
		fmt.Printf("  Download: %s\n", result.UpdateURL)
	}

	fmt.Printf("\n")
}
