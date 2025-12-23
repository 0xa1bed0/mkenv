package dockerfile

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/utils"
)

type Dockerfile []string

func (df Dockerfile) String() string {
	out := ""
	for _, line := range df {
		out += line + "\n"
	}
	return out
}

func (plan *BuildPlan) GenerateDockerfile() Dockerfile {
	lines := Dockerfile{}

	// Base image
	if plan.baseImage == "" {
		plan.baseImage = "debian:bookworm-slim"
	}
	lines = append(lines, "# ───────────────────────────────────────────")
	lines = append(lines, "# SYSTEM BASE IMAGE (SECURITY-ALLOWED)")
	lines = append(lines, fmt.Sprintf("FROM %s", plan.baseImage))

	// Root env
	if len(plan.envs) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# ENVIRONMENT")
		ks := utils.SortedKeys(plan.envs)
		for _, k := range ks {
			lines = append(lines, fmt.Sprintf("ENV %s=%s", k, replaceVars(plan.envs[k], plan.args)))
		}
	}

	// Buildtime tmp workdir for root steps
	lines = append(lines, "", "# ───────────────────────────────────────────")
	lines = append(lines, "# TMP BUILD TIME WORKDIR (root scope)")
	lines = append(lines, "WORKDIR /tmp/build/root")

	// Root steps (exec-form RUN)
	if len(plan.rootRun) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# ROOT-LEVEL SETUP STEPS (exec form)")
		for _, cmd := range plan.rootRun {
			if cmd.When == "build" {
				lines = append(lines, "RUN "+jsonExec(cmd.Argv, plan.args))
			}
		}
	}

	// make user scoped build time temp dir
	lines = append(lines, `RUN ["mkdir", "-p", "/tmp/build/user"]`)
	lines = append(lines, `RUN ["chown", "`+plan.args["MKENV_USERNAME"]+`:`+plan.args["MKENV_USERNAME"]+`", "/tmp/build/user"]`)

	// Switch to non-root user
	lines = append(lines, "", "# ───────────────────────────────────────────")
	lines = append(lines, "# DEFAULT USER (NON-ROOT) — SECURITY REQUIREMENT")
	lines = append(lines, fmt.Sprintf("USER %s", plan.args["MKENV_USERNAME"]))

	// Buildtime tmp workdir for user steps
	lines = append(lines, "", "# ───────────────────────────────────────────")
	lines = append(lines, "# TMP BUILD TIME WORKDIR (User scope)")
	lines = append(lines, "WORKDIR /tmp/build/user")

	// User-level steps
	if len(plan.userRun) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# USER-LEVEL BUILD STEPS (exec form)")
		for _, cmd := range plan.userRun {
			if cmd.When == "build" {
				// TODO: think of build args isolation from user commands
				lines = append(lines, "RUN "+jsonExec(cmd.Argv, plan.args))
			}
		}
	}

	// RC appends via heredoc (append-only, deterministic)
	if len(plan.fileTemplates) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# FILE APPENDS (MERGED, HEREDOC, APPEND-ONLY)")
		for _, fileTemplate := range plan.fileTemplates {
			rcFilePath := "${MKENV_HOME}/.mkenvrc"
			if fileTemplate.FilePath != "rc" {
				rcFilePath = fileTemplate.FilePath
			}
			lines = append(lines, heredocAppend(fileTemplate, rcFilePath, plan.args))
		}
	}

	lines = append(lines, "", "# ───────────────────────────────────────────")
	lines = append(lines, "# WORKDIR")
	lines = append(lines, fmt.Sprintf("WORKDIR %s", "/workdir"))

	cacheFoldersPaths := []string{}
	for _, cp := range plan.cachePaths {
		path := replaceVars(cp, plan.args)
		cacheFoldersPaths = append(cacheFoldersPaths, path)
		lines = append(lines, `RUN ["mkdir", "-p", "`+path+`"]`)
	}

	// Entrypoint/Cmd
	if len(plan.entrypoint) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# ENTRYPOINT (exec form)")
		lines = append(lines, "ENTRYPOINT "+jsonExec(plan.entrypoint, plan.args))
		lines = append(lines, fmt.Sprintf("LABEL mkenv.attachInstruction=%s", strings.Join(plan.attachInstruction, "|MKENVSEP|")))
	}
	if len(plan.cmd) > 0 {
		lines = append(lines, "", "# CMD (exec form)")
		lines = append(lines, "CMD "+jsonExec(plan.cmd, plan.args))
	}

	// Audit label
	if len(plan.order) > 0 {
		uniq := bricksengine.ToStrings(plan.order)
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# AUDIT LABELS")
		lines = append(lines, fmt.Sprintf("LABEL mkenv.bricks=\"%s\"", strings.Join(uniq, ",")))
	}

	if len(cacheFoldersPaths) > 0 {
		lines = append(lines, fmt.Sprintf("LABEL mkenv_cache_volumes=\"%s\"", strings.Join(cacheFoldersPaths, ",")))
	}

	lines = append(lines, "LABEL mkenv=true")

	return lines
}

func replaceVars(input string, vars map[string]string) string {
	// Regex to match ${VAR_NAME}
	re := regexp.MustCompile(`\$\{([^}]+)\}`)

	return re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract key inside ${...}
		key := re.FindStringSubmatch(match)[1]
		if val, ok := vars[key]; ok {
			return val
		}
		// If key not found, keep original match
		return match
	})
}

func jsonExec(argv []string, buildArgs map[string]string) string {
	b, _ := json.Marshal(argv)

	// TODO: make audit trail on what args were used?
	// TODO: to think: lets replace it on planner?
	return replaceVars(string(b), buildArgs)
}

func sanitizeHeredocID(s string) string {
	if s == "" {
		s = "RC"
	}
	var b strings.Builder
	b.WriteString("MKENV_")
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func heredocAppend(f bricksengine.FileTemplate, filePath string, buildArgs map[string]string) string {
	id := sanitizeHeredocID(f.ID)
	targetFile := replaceVars(filePath, buildArgs)
	content := replaceVars(f.Content, buildArgs)

	payload := fmt.Sprintf("cat >> %s <<\"%s\"\n%s\n%s", targetFile, id, content, id)

	return "RUN " + jsonExec([]string{"/bin/sh", "-lc", payload}, buildArgs)
}
