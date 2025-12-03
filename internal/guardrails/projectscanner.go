package guardrails

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/0xa1bed0/mkenv/internal/logs"
)

// Suspicious file indicators
var suspiciousFilenames = []string{
	"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
	"credentials.json", "auth.json", "vault.json",
	"token", "secrets", "apikey", "api_key", "access_token", "refresh_token",
	"azureProfile.json", "azureProfile", "aws_credentials", "gcloud.json",
	"netrc", ".env", ".env.production", ".env.development",
	"docker.env", ".npmrc", ".pypirc", ".dockercfg", ".dockerconfigjson",
	".npm_token", ".yarnrc", ".git-credentials", ".gitconfig", ".git_token",
	".github_token", ".gh_token", ".github_credentials", "github.env",
	"secrets.env", ".secrets", ".env.secret", ".env.secrets", ".env.vault",
	"vault.env", ".vault-token", ".env.staging", ".env.prod",
	"docker-compose.override.yml",
	".docker-secrets", ".docker-login.json", "kubeconfig", "kube_config.yaml",
	".helm/values.yaml", ".keypass", ".aws_secrets", ".gcp_keys.json",
	".ftpconfig",
}

var suspiciousContentRegexps = []*regexp.Regexp{
	// --- Private keys / SSH ---
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`ssh-(rsa|ed25519|dss) `),

	// --- GitHub / GitLab ---
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36}`),
	regexp.MustCompile(`glpat-[A-Za-z0-9\-]{20,}`),

	// --- Stripe ---
	regexp.MustCompile(`sk_live_[0-9A-Za-z]{24}`),
	regexp.MustCompile(`rk_live_[0-9A-Za-z]{24}`),

	// --- Slack ---
	regexp.MustCompile(`xox[baprs]-[0-9A-Za-z]{10,48}`),

	// --- AWS ---
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                                                // Access key id
	regexp.MustCompile(`(?i)aws(.{0,20})?(secret|key)[^A-Za-z0-9]{0,3}[A-Za-z0-9/+]{40}`), // Secret key

	// --- Google / Firebase / GCP ---
	regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`), // Google API key (used by many GCP/Gemini/Firebase products)
	regexp.MustCompile(`(?i)\b(gcp|google)(.{0,20})?(api[_-]?key|token|secret)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),

	// --- OpenAI / Azure OpenAI / OpenRouter ---
	regexp.MustCompile(`\bsk-[A-Za-z0-9]{32,}\b`),   // OpenAI-style keys (prefix stable, length can vary)
	regexp.MustCompile(`\b(or-[A-Za-z0-9]{20,})\b`), // OpenRouter keys often start with or-
	regexp.MustCompile(`(?i)\b(openai|azure[_-]?openai|openrouter)(.{0,20})?(key|token|secret)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),

	// --- Anthropic (Claude) ---
	regexp.MustCompile(`\bsk-ant-[A-Za-z0-9\-_]{20,}\b`),
	regexp.MustCompile(`(?i)\b(anthropic|claude)(.{0,20})?(key|token|secret)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),

	// --- Cohere ---
	regexp.MustCompile(`(?i)\b(cohere)(.{0,20})?(api[_-]?key|token|secret)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),

	// --- Hugging Face ---
	regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}\b`),
	regexp.MustCompile(`(?i)\b(huggingface|hf)(.{0,20})?(token|key|secret)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),

	// --- Mistral / Together / Groq / Replicate / Fireworks / Perplexity / DeepInfra / AI21 ---
	regexp.MustCompile(`(?i)\b(mistral|together|groq|replicate|fireworks|perplexity|deepinfra|ai21)(.{0,20})?(api[_-]?key|token|secret)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),

	// --- Cloudflare ---
	regexp.MustCompile(`(?i)\b(CF_API_KEY|CF_API_TOKEN|CLOUDFLARE_API_TOKEN)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),

	// --- Vercel / Netlify / Render / Fly.io ---
	regexp.MustCompile(`(?i)\b(VERCEL_TOKEN|NETLIFY_AUTH_TOKEN|RENDER_API_KEY|FLY_API_TOKEN)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),

	// --- Supabase / PlanetScale / Neon / Railway ---
	regexp.MustCompile(`(?i)\b(SUPABASE_SERVICE_ROLE_KEY|SUPABASE_ANON_KEY)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),
	regexp.MustCompile(`(?i)\b(PLANETSCALE|NEON|RAILWAY)(.{0,20})?(token|key|secret)\b.{0,5}['"]?[A-Za-z0-9\-_]{20,}`),

	// --- Postgres/MySQL/Mongo/Redis connection strings ---
	regexp.MustCompile(`(?i)\b(postgres(ql)?|mysql|mongodb(\+srv)?|redis)://[^ \n'"]+`),

	// --- JWT (only real 3-part JWTs) ---
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}`),
}

var ignoredDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	".venv":        true,
	".tox":         true,
	"__pycache__":  true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".next":        true,
	".cache":       true,
	".gradle":      true,
	".idea":        true,
	".vscode":      true,
	".terraform":   true,
}

const maxFileSizeForScan = 5 * 1024 * 1024 // 5 MB

type SensitivityWarning struct {
	Path    string
	Reason  string
	Content []string
}

var ErrScanCanceled = errors.New("scan canceled")

// ScanSuspiciousFiles walks a path tree and reports files that appear suspicious by name or content.
func ScanSuspiciousFiles(ctx context.Context, root string) ([]*SensitivityWarning, error) {
	suspicious := []*SensitivityWarning{}
	tailBox := logs.NewTailBox("Files scanner")
	defer tailBox.Close()

	// helper to keep ctx checks short + consistent
	checkCtx := func() error {
		if err := ctx.Err(); err != nil {
			return ErrScanCanceled
		}
		return nil
	}

	// in case ctx is already canceled
	if err := checkCtx(); err != nil {
		return nil, err
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}

		if ctxErr := checkCtx(); ctxErr != nil {
			return ctxErr
		}

		if d.IsDir() {
			base := filepath.Base(path)
			if ignoredDirs[base] {
				tailBox.Printf("Skipping %s folder", base)
				return filepath.SkipDir
			}
			return nil
		}

		tailBox.Printf("Check filename %s if it potentially sensitive...", path)

		lower := strings.ToLower(filepath.Base(path))
		for _, name := range suspiciousFilenames {
			if strings.Contains(lower, name) {
				suspicious = append(suspicious, &SensitivityWarning{
					Path:   path,
					Reason: "Filename indicates potential sensitivity",
				})
				return nil
			}
		}

		info, err := d.Info()
		if err != nil || info.Size() > maxFileSizeForScan {
			return nil
		}

		tailBox.Printf("Check if %s has potentially sensitive data...", path)

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), int(maxFileSizeForScan))
		previousLine := ""
		for scanner.Scan() {
			line := scanner.Text()
			if !utf8.ValidString(line) {
				continue
			}
			for _, re := range suspiciousContentRegexps {
				if re.MatchString(line) {
					// Peek one line ahead (only if it exists)
					nextLine := ""
					if scanner.Scan() {
						nextLine = scanner.Text()
						if !utf8.ValidString(nextLine) {
							nextLine = ""
						}
					} else if err := scanner.Err(); err != nil {
						// ignore scan error for context, we're already flagging
						nextLine = ""
					}

					suspicious = append(suspicious, &SensitivityWarning{
						Path:   path,
						Reason: fmt.Sprintf("file contains potentially sensitive data: %s", re.String()),
						Content: []string{
							previousLine,
							line,
							nextLine,
						},
					})
					return nil
				}
			}

			previousLine = line
		}
		return nil
	})
	return suspicious, err
}
