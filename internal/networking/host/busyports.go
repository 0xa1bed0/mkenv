package host

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

// GetHostBusyPorts scans the host for all listening TCP ports.
// For now it's implemented for macOS via `lsof` and returns (ports, generatedAt).
// No DB, no locking – each mkenv process just runs its own scan.
func GetHostBusyPorts(rt *runtime.Runtime) ([]int, error) {
	switch rt.GOOS() {
	case "darwin":
		ports, err := scanBusyPortsLsof(rt.Ctx())
		if err != nil {
			return nil, err
		}
		// Dedup + sort for stable output
		if len(ports) > 1 {
			sort.Ints(ports)
			out := ports[:0]
			last := -1
			for _, p := range ports {
				if p != last {
					out = append(out, p)
					last = p
				}
			}
			ports = out
		}
		return ports, nil
	default:
		// TODO: implement for non mac
		logs.Warnf("hostports: GetHostBusyPorts not implemented for GOOS=%s, returning empty set", rt.GOOS())
		return nil, nil
	}
}

// macOS: use `lsof -nP -iTCP -sTCP:LISTEN` to get all listening TCP ports.
func scanBusyPortsLsof(ctx context.Context) ([]int, error) {
	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-iTCP", "-sTCP:LISTEN")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("lsof stdout pipe: %w", err)
	}

	// Optional: capture stderr for debugging instead of letting it disappear
	// var stderr bytes.Buffer
	// cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("lsof start: %w", err)
	}

	portsSet := make(map[int]struct{})

	scanner := bufio.NewScanner(stdout)

	// Regex: colon followed by 1–5 digits, e.g. ":80", ":3000", ":62468"
	portRe := regexp.MustCompile(`:(\d{1,5})\b`)

	for scanner.Scan() {
		line := scanner.Text()

		// Be conservative: only look at TCP LISTEN lines.
		if !strings.Contains(line, "TCP") {
			continue
		}
		if !strings.Contains(line, "LISTEN") {
			continue
		}

		matches := portRe.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			portStr := m[1]
			p, err := strconv.Atoi(portStr)
			if err != nil || p <= 0 || p > 65535 {
				continue
			}
			portsSet[p] = struct{}{}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("lsof scan: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		// If context was canceled, CommandContext kills lsof; ignore in that case.
		if ctx.Err() == nil {
			// Uncomment if you captured stderr:
			// logs.Warnf("hostports: lsof exited with error: %v, stderr: %s", err, stderr.String())
			logs.Warnf("hostports: lsof exited with error: %v", err)
		}
	}

	out := make([]int, 0, len(portsSet))
	for p := range portsSet {
		out = append(out, p)
	}
	sort.Ints(out)
	return out, nil
}
