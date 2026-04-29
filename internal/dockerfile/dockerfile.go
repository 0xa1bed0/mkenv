package dockerfile

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/bricksengine"
	"github.com/0xa1bed0/mkenv/internal/utils"
	"github.com/0xa1bed0/mkenv/internal/version"
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

	// Merged ENV instruction
	if len(plan.envs) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# ENVIRONMENT")
		ks := utils.SortedKeys(plan.envs)
		var envParts []string
		for _, k := range ks {
			envParts = append(envParts, fmt.Sprintf("%s=%s", k, replaceVars(plan.envs[k], plan.args)))
		}
		lines = append(lines, "ENV "+strings.Join(envParts, " "))
	}

	// Buildtime tmp workdir for root steps
	lines = append(lines, "", "# ───────────────────────────────────────────")
	lines = append(lines, "# TMP BUILD TIME WORKDIR (root scope)")
	lines = append(lines, "WORKDIR /tmp/build/root")

	// Root steps (compacted RUN)
	if len(plan.rootRun) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# ROOT-LEVEL SETUP STEPS")
		lines = append(lines, compactRuns(plan.rootRun, plan.args)...)
	}

	// make user scoped build time temp dir (merged into single RUN)
	username := plan.args["MKENV_USERNAME"]
	lines = append(lines, "RUN mkdir -p /tmp/build/user && chown "+username+":"+username+" /tmp/build/user")

	// Switch to non-root user
	lines = append(lines, "", "# ───────────────────────────────────────────")
	lines = append(lines, "# DEFAULT USER (NON-ROOT) — SECURITY REQUIREMENT")
	lines = append(lines, fmt.Sprintf("USER %s", username))

	// Buildtime tmp workdir for user steps
	lines = append(lines, "", "# ───────────────────────────────────────────")
	lines = append(lines, "# TMP BUILD TIME WORKDIR (User scope)")
	lines = append(lines, "WORKDIR /tmp/build/user")

	// User-level steps (compacted RUN)
	if len(plan.userRun) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# USER-LEVEL BUILD STEPS")
		lines = append(lines, compactRuns(plan.userRun, plan.args)...)
	}

	// RC appends via heredoc (merged by target file)
	if len(plan.fileTemplates) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# FILE APPENDS (MERGED, HEREDOC, APPEND-ONLY)")
		lines = append(lines, mergeFileTemplates(plan.fileTemplates, plan.args)...)
	}

	lines = append(lines, "", "# ───────────────────────────────────────────")
	lines = append(lines, "# WORKDIR")
	lines = append(lines, fmt.Sprintf("WORKDIR %s", plan.args["MKENV_WORKDIR"]))

	cacheFoldersPaths := []string{}
	for _, cp := range plan.cachePaths {
		path := replaceVars(cp, plan.args)
		cacheFoldersPaths = append(cacheFoldersPaths, path)
	}
	if len(cacheFoldersPaths) > 0 {
		lines = append(lines, `RUN ["mkdir", "-p", "`+strings.Join(cacheFoldersPaths, `", "`)+`"]`)
	}

	// Cache files are stored in a dedicated volume directory and symlinked
	// to their expected paths. This works around Docker volumes being directories.
	// All cached files go into a single directory that gets mounted as a volume.
	cacheFileStoreDir := ""
	if len(plan.cacheFilePaths) > 0 {
		cacheFileStoreDir = replaceVars("${MKENV_HOME}/.mkenv-file-cache", plan.args)

		// Build a single shell script for all cache file operations
		var parentDirs []string
		var touchCmds []string
		var linkCmds []string

		for _, cf := range plan.cacheFilePaths {
			originalPath := replaceVars(cf, plan.args)
			fileName := originalPath[strings.LastIndex(originalPath, "/")+1:]
			cachedPath := cacheFileStoreDir + "/" + fileName
			parentDir := originalPath[:strings.LastIndex(originalPath, "/")]

			parentDirs = append(parentDirs, parentDir)
			touchCmds = append(touchCmds, cachedPath)
			linkCmds = append(linkCmds, "ln -sf "+cachedPath+" "+originalPath)
		}

		// Combine: mkdir all dirs, touch all files, create all symlinks
		allDirs := append([]string{cacheFileStoreDir}, parentDirs...)
		cmds := []string{
			"mkdir -p " + strings.Join(allDirs, " "),
			"touch " + strings.Join(touchCmds, " "),
		}
		cmds = append(cmds, linkCmds...)

		lines = append(lines, `RUN /bin/sh -c '`+strings.Join(cmds, " && ")+`'`)
	}

	// Entrypoint/Cmd
	if len(plan.entrypoint) > 0 {
		lines = append(lines, "", "# ───────────────────────────────────────────")
		lines = append(lines, "# ENTRYPOINT (exec form)")
		lines = append(lines, "ENTRYPOINT "+jsonExec(plan.entrypoint, plan.args))
	}
	if len(plan.cmd) > 0 {
		lines = append(lines, "", "# CMD (exec form)")
		lines = append(lines, "CMD "+jsonExec(plan.cmd, plan.args))
	}

	// Collect all labels into a single LABEL instruction
	labels := make(map[string]string)

	if len(plan.entrypoint) > 0 {
		labels["mkenv.attachInstruction"] = strings.Join(plan.attachInstruction, "|MKENVSEP|")
	}
	if len(plan.order) > 0 {
		uniq := bricksengine.ToStrings(plan.order)
		labels["mkenv.bricks"] = fmt.Sprintf(`"%s"`, strings.Join(uniq, ","))
	}
	if len(cacheFoldersPaths) > 0 {
		labels["mkenv_cache_volumes"] = fmt.Sprintf(`"%s"`, strings.Join(cacheFoldersPaths, ","))
	}
	if cacheFileStoreDir != "" {
		labels["mkenv_cache_file_store"] = fmt.Sprintf(`"%s"`, cacheFileStoreDir)
	}
	labels[version.ImageSchemaVersionLabel] = fmt.Sprintf(`"%d"`, version.ImageSchemaVersion)
	labels["mkenv"] = "true"

	lines = append(lines, "", "# ───────────────────────────────────────────")
	lines = append(lines, "# LABELS")
	labelKeys := utils.SortedKeys(labels)
	var labelParts []string
	for _, k := range labelKeys {
		labelParts = append(labelParts, k+"="+labels[k])
	}
	lines = append(lines, "LABEL "+strings.Join(labelParts, " "))

	return lines
}

// shellQuoteArg quotes a string for safe use in a shell command.
// If the string contains only safe characters, it is returned as-is.
// Otherwise it is single-quoted with any embedded single quotes escaped.
func shellQuoteArg(s string) string {
	safe := true
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '-' || r == '_' || r == '/' || r == '.' || r == ':' || r == '=' || r == ',' || r == '+' || r == '@') {
			safe = false
			break
		}
	}
	if safe && len(s) > 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// isShellCommand returns true if the command's Argv[0] is a shell interpreter.
func isShellCommand(cmd bricksengine.Command) bool {
	if len(cmd.Argv) == 0 {
		return false
	}
	switch cmd.Argv[0] {
	case "sh", "/bin/sh", "bash", "/bin/bash":
		return true
	}
	return false
}

// compactRuns groups consecutive simple (non-shell) build commands into merged
// shell-form RUN lines. Shell-invoked commands are kept as individual exec-form
// RUN lines since they already contain combined logic internally.
func compactRuns(cmds []bricksengine.Command, buildArgs map[string]string) []string {
	var lines []string
	var pending []string // accumulated simple command strings

	flush := func() {
		if len(pending) > 0 {
			lines = append(lines, "RUN "+strings.Join(pending, " && "))
			pending = nil
		}
	}

	for _, cmd := range cmds {
		if cmd.When != "build" {
			continue
		}
		if isShellCommand(cmd) {
			flush()
			lines = append(lines, "RUN "+jsonExec(cmd.Argv, buildArgs))
		} else {
			// Build shell-form command from argv
			var parts []string
			for _, arg := range cmd.Argv {
				parts = append(parts, shellQuoteArg(replaceVars(arg, buildArgs)))
			}
			pending = append(pending, strings.Join(parts, " "))
		}
	}
	flush()
	return lines
}

// mergeFileTemplates groups file templates by target file path and combines
// templates targeting the same file into a single heredoc append.
func mergeFileTemplates(templates []bricksengine.FileTemplate, buildArgs map[string]string) []string {
	if len(templates) == 0 {
		return nil
	}

	// Preserve order of first occurrence of each target path
	var orderedPaths []string
	grouped := make(map[string][]bricksengine.FileTemplate)

	for _, ft := range templates {
		targetPath := ft.FilePath
		if targetPath == "rc" {
			targetPath = "${MKENV_HOME}/.mkenvrc"
		}
		targetPath = replaceVars(targetPath, buildArgs)

		if _, exists := grouped[targetPath]; !exists {
			orderedPaths = append(orderedPaths, targetPath)
		}
		grouped[targetPath] = append(grouped[targetPath], ft)
	}

	var lines []string
	for _, targetPath := range orderedPaths {
		fts := grouped[targetPath]
		// Combine all contents for same target file
		var combinedContent strings.Builder
		lastID := fts[0].ID
		for i, ft := range fts {
			content := replaceVars(ft.Content, buildArgs)
			if i > 0 {
				combinedContent.WriteString("\n")
			}
			combinedContent.WriteString(content)
			lastID = ft.ID
		}

		id := sanitizeHeredocID(lastID)
		payload := fmt.Sprintf("cat >> %s <<\"%s\"\n%s\n%s", targetPath, id, combinedContent.String(), id)
		lines = append(lines, "RUN "+jsonExec([]string{"/bin/sh", "-lc", payload}, buildArgs))
	}
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
