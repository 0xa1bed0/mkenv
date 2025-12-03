package sandbox

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/networking/shared"
)

// CollectSnapshot returns a best-effort view of all listening TCP/UDP
// sockets inside the container. It only works on Linux (/proc).
func CollectSnapshot() (shared.Snapshot, error) {
	snap := shared.Snapshot{
		Listeners: map[int]shared.Listener{},
	}

	// 1) Parse /proc/net/{tcp,tcp6,udp,udp6} into inode->Listener (without PID/Cmd).
	inodeMap := make(map[uint64]*shared.Listener)

	if err := addProtoFromProc("/proc/net/tcp", shared.ProtoTCP, inodeMap); err != nil {
		logs.Warnf("can't list opened tcp ports. error: %v", err)
	}
	if err := addProtoFromProc("/proc/net/tcp6", shared.ProtoTCP, inodeMap); err != nil {
		logs.Warnf("can't list opened tcp6 ports. error: %v", err)
	}
	if err := addProtoFromProc("/proc/net/udp", shared.ProtoUDP, inodeMap); err != nil {
		logs.Warnf("can't list opened udp ports. error: %v", err)
	}
	if err := addProtoFromProc("/proc/net/udp6", shared.ProtoUDP, inodeMap); err != nil {
		logs.Warnf("can't list opened udp6 ports. error: %v", err)
	}

	if len(inodeMap) == 0 {
		return snap, nil
	}

	// 2) Walk /proc/<pid>/fd to map inode -> PID + Cmd.
	mapInodesToPIDs(inodeMap)

	// PID of the current process
	selfPID := os.Getpid()

	// 3) Flatten map to slice.
	for _, l := range inodeMap {
		// Ignore entries that don't have a port (parsing failure).
		if l.Port <= 0 {
			continue
		}
		// Exclude listeners created by this process itself.
		if l.PID == selfPID {
			continue
		}
		snap.Listeners[l.Port] = *l
	}

	return snap, nil
}

// addProtoFromProc parses /proc/net/{tcp,udp,...} and fills inodeMap
// with listening sockets (state=0A for TCP, for UDP we don't filter by state).
func addProtoFromProc(path string, proto shared.Proto, inodeMap map[uint64]*shared.Listener) error {
	f, err := os.Open(path)
	if err != nil {
		// file might not exist (e.g. no IPv6), not fatal
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Skip header
	if !scanner.Scan() {
		return nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		// Expected minimal fields: sl, local_address, rem_address, st, ..., uid, timeout, inode,...
		// Indexes: 0:sl, 1:local_address, 2:rem_address, 3:st, 7:uid, 9:inode (for tcp)
		if len(fields) < 10 {
			continue
		}

		localAddr := fields[1]
		state := fields[3]
		uidStr := fields[7]
		inodeStr := fields[9]

		// TCP: we only care about LISTEN (0A).
		if proto == shared.ProtoTCP && state != "0A" {
			continue
		}

		ip, port, err := parseProcAddress(localAddr)
		if err != nil {
			continue
		}

		uid, _ := strconv.Atoi(uidStr)
		inode, err := strconv.ParseUint(inodeStr, 10, 64)
		if err != nil {
			continue
		}

		// Some kernels may show 0.0.0.0 as "00000000" or "::".
		if ip == "" {
			if proto == shared.ProtoTCP || proto == shared.ProtoUDP {
				ip = "0.0.0.0"
			}
		}

		inodeMap[inode] = &shared.Listener{
			Port:  port,
			Proto: proto,
			UID:   uid,
		}
	}

	return nil
}

// parseProcAddress parses the hex IP:PORT from /proc/net/* format.
// Example local_address: "0100007F:0BB8" (127.0.0.1:3000).
func parseProcAddress(s string) (string, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("bad local_address: %q", s)
	}
	ipHex, portHex := parts[0], parts[1]

	port64, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return "", 0, err
	}
	port := int(port64)

	// IPv4 is 8 hex chars, IPv6 is 32 hex chars.
	switch len(ipHex) {
	case 8:
		// IPv4 is stored little-endian.
		// E.g. 0100007F -> 127.0.0.1
		var b [4]byte
		for i := 0; i < 4; i++ {
			x, err := strconv.ParseUint(ipHex[2*i:2*i+2], 16, 8)
			if err != nil {
				return "", 0, err
			}
			b[i] = byte(x)
		}
		// reverse
		ip := net.IPv4(b[3], b[2], b[1], b[0]).String()
		return ip, port, nil
	case 32:
		// Simplified IPv6 parsing: we don't really care about exact textual form
		// for your use case; you mostly care about the port and proto.
		// We'll return "::" to indicate "some IPv6 address".
		return "::", port, nil
	default:
		return "", port, nil
	}
}

// mapInodesToPIDs walks /proc/<pid>/fd and resolves socket:[inode] symlinks
// to associate inodes with processes and command names.
func mapInodesToPIDs(inodeMap map[uint64]*shared.Listener) {
	// Build a quick set of inodes we care about.
	inodesOfInterest := make(map[uint64]struct{}, len(inodeMap))
	for ino := range inodeMap {
		inodesOfInterest[ino] = struct{}{}
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}

		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fdEntries, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fd := range fdEntries {
			linkPath := filepath.Join(fdDir, fd.Name())
			target, err := os.Readlink(linkPath)
			if err != nil {
				continue
			}
			// Format: "socket:[12345]"
			if !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
				continue
			}
			inoStr := target[len("socket:[") : len(target)-1]
			ino, err := strconv.ParseUint(inoStr, 10, 64)
			if err != nil {
				continue
			}
			if _, ok := inodesOfInterest[ino]; !ok {
				continue
			}

			l := inodeMap[ino]
			// Only record the first PID we find to avoid bouncing.
			if l.PID != 0 {
				continue
			}
			l.PID = pid
			l.Cmd = readProcCmd(pid)
		}
	}
}

func readProcCmd(pid int) string {
	// Try /proc/<pid>/comm first (single word, no args).
	commPath := filepath.Join("/proc", strconv.Itoa(pid), "comm")
	if b, err := os.ReadFile(commPath); err == nil && len(b) > 0 {
		return strings.TrimSpace(string(b))
	}
	// Fallback to /proc/<pid>/cmdline (argv split by NUL).
	cmdPath := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	b, err := os.ReadFile(cmdPath)
	if err != nil || len(b) == 0 {
		return ""
	}
	parts := strings.Split(string(b), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return filepath.Base(parts[0])
}
