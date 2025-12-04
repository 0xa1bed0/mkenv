package guardrails

import (
	"os/user"
	"path/filepath"
	"strings"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/utils"
)

// A forbidden rule: either exact path, prefix path, or glob.
type forbiddenRule struct {
	Path    string // normalized absolute path or pattern
	Exact   bool   // forbid ONLY this exact path
	Prefix  bool   // forbid this path AND any child paths
	Pattern bool   // forbid paths matching a glob-like pattern
}

var forbiddenRules []forbiddenRule

func init() {
	home := mustHome()

	expand := func(p string) string {
		if strings.HasPrefix(p, "~/") {
			return filepath.Join(home, p[2:])
		}
		return p
	}

	raw := []forbiddenRule{
		// --- LINUX & MACOS SYSTEM DIRECTORIES ---
		{Path: "/bin", Prefix: true},
		{Path: "/sbin", Prefix: true},
		{Path: "/lib", Prefix: true},
		{Path: "/lib32", Prefix: true},
		{Path: "/lib64", Prefix: true},
		{Path: "/usr", Prefix: true},
		{Path: "/usr/local", Prefix: true},
		{Path: "/etc", Prefix: true},
		{Path: "/dev", Prefix: true},
		{Path: "/proc", Prefix: true},
		{Path: "/sys", Prefix: true},
		{Path: "/run", Prefix: true},
		{Path: "/var", Prefix: true},
		{Path: "/opt", Prefix: true},
		{Path: "/snap", Prefix: true},
		{Path: "/media", Prefix: true},
		{Path: "/mnt", Prefix: true},
		{Path: "/root", Prefix: true},
		{Path: "/boot", Prefix: true},
		{Path: "/srv", Prefix: true},
		{Path: "/tmp", Prefix: true},
		{Path: "/lost+found", Prefix: true},

		// --- CONTAINER SOCKETS ---
		{Path: "/var/run/docker.sock", Exact: true},
		{Path: "/run/docker.sock", Exact: true},
		{Path: "/var/run/podman/podman.sock", Exact: true},
		{Path: "/run/podman/podman.sock", Exact: true},
		{Path: "/var/run/containerd/containerd.sock", Exact: true},
		{Path: "/run/containerd/containerd.sock", Exact: true},

		// --- MACOS SYSTEM DIRECTORIES ---
		{Path: "/System", Prefix: true},
		{Path: "/System/Library", Prefix: true},
		{Path: "/System/Applications", Prefix: true},
		{Path: "/Applications", Prefix: true},
		{Path: "/Library", Prefix: true},
		{Path: "/private", Prefix: true},
		{Path: "/Volumes", Prefix: true},

		// --- MACOS USER-SENSITIVE DIRECTORIES ---
		{Path: expand("~/Library"), Prefix: true},

		// --- WINDOWS via WSL (/mnt/*) ---
		{Path: "/mnt", Prefix: true},

		// --- USER-SENSITIVE PATHS ---
		{Path: expand("~/.ssh"), Prefix: true},
		{Path: expand("~/.gnupg"), Prefix: true},
		{Path: expand("~/.pki"), Prefix: true},
		{Path: expand("~/.aws"), Prefix: true},
		{Path: expand("~/.azure"), Prefix: true},
		{Path: expand("~/.docker"), Prefix: true},
		{Path: expand("~/.kube"), Prefix: true},
		{Path: expand("~/.git-credentials"), Exact: true},
		{Path: expand("~/.config/gh"), Prefix: true},
		{Path: expand("~/.config/gcloud"), Prefix: true},
		{Path: expand("~/.config/doctl"), Prefix: true},
		{Path: expand("~/.config/hcloud"), Prefix: true},
		{Path: expand("~/.config/scw"), Prefix: true},
		{Path: expand("~/.config/linode"), Prefix: true},
		{Path: expand("~/.local/share/keyrings"), Prefix: true},
		{Path: expand("~/.config/Bitwarden"), Prefix: true},
		{Path: expand("~/.config/1Password"), Prefix: true},
		{Path: expand("~/.config/Enpass"), Prefix: true},
		{Path: expand("~/.keepass"), Prefix: true, Pattern: true},
		{Path: expand("~/.mozilla"), Prefix: true},
		{Path: expand("~/.config/google-chrome"), Prefix: true},
		{Path: expand("~/.config/chromium"), Prefix: true},
		{Path: expand("~/.config/BraveSoftware"), Prefix: true},
		{Path: expand("~/.config/microsoft-edge"), Prefix: true},

		// --- MKENV INTERNALS ---
		{Path: expand(hostappconfig.ConfigBasePath()), Prefix: true},
	}

	for _, r := range raw {
		r.Path = filepath.Clean(r.Path)
		forbiddenRules = append(forbiddenRules, r)
	}
}

func mustHome() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return usr.HomeDir
}

func IsAbsolutelyForbidden(rawPath string) bool {
	if rawPath == "" {
		rawPath = "."
	}

	p, err := utils.ResolveFolderStrict(rawPath)
	if err != nil {
		logs.Errorf("[guardrails] can't resolve path %s. error:%v", rawPath, err)
		return true
	}

	for _, rule := range forbiddenRules {
		r := rule.Path

		if rule.Exact && p == r {
			logs.Warnf("path %s is forbidden globally path %s", p, r)
			return true
		}
		if rule.Prefix {
			if IsUnderPrefix(r, p) {
				logs.Warnf("path %s is under forbidden globally path %s", p, r)
				return true
			}
		}
		if rule.Pattern {
			if strings.HasSuffix(r, "*") {
				prefix := strings.TrimSuffix(r, "*")
				if strings.HasPrefix(p, prefix) {
					logs.Warnf("path %s is forbidden globally path %s", p, prefix)
					return true
				}
			}
		}
	}

	return false
}

func IsUnderPrefix(base, path string) bool {
	var err error
	path, err = utils.ResolvePathStrict(path)
	if err != nil {
		return false
	}

	var rel string
	rel, err = filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}
