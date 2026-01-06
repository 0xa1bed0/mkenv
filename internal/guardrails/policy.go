package guardrails

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/bricksengine"
)

type policy struct {
	DisableBricks_      []bricksengine.BrickID                     `json:"disabled_bricks"`
	EnableBricks_       []bricksengine.BrickID                     `json:"enabled_bricks"`
	DisableAuto_        bool                                       `json:"disable_auto"`
	BricksConfigs_      map[bricksengine.BrickID]map[string]string `json:"bricks_config"`
	AllowedMounts_      []string                                   `json:"allowed_mount_paths"`  // if empty - allow all except forbidden globally
	AllowedProjectRoot_ string                                     `json:"allowed_project_path"` // if empty - allow all except forbidden globally
	IgnorePreferences_  bool                                       `json:"ignore_preferences"`
	ReverseProxy_       *ReverseProxyPolicy                        `json:"reverse_proxy"`
}

// ReverseProxyPolicy controls which host ports can be accessed from the container
type ReverseProxyPolicy struct {
	CustomDeniedPorts  []int `json:"denied_ports"`  // Additional ports to deny beyond hardcoded list
	CustomAllowedPorts []int `json:"allowed_ports"` // Ports to allow despite being in deny lists
}

// AllowedMounts implements Policy.
func (p *policy) AllowedMounts() []string {
	out := make([]string, len(p.AllowedMounts_))
	for i, val := range p.AllowedMounts_ {
		out[i] = val
	}
	return out
}

// AllowedProjectRoot implements Policy.
func (p *policy) AllowedProjectRoot() string {
	return p.AllowedProjectRoot_
}

// BricksConfigs implements Policy.
func (p *policy) BricksConfigs() map[bricksengine.BrickID]map[string]string {
	out := make(map[bricksengine.BrickID]map[string]string, len(p.BricksConfigs_))

	for brickId, config := range p.BricksConfigs_ {
		out[brickId] = make(map[string]string, len(config))
		for k, v := range config {
			out[brickId][k] = v
		}
	}

	return out
}

// DisableAuto implements Policy.
func (p *policy) DisableAuto() bool {
	return p.DisableAuto_
}

// DisableBricks implements Policy.
func (p *policy) DisableBricks() []bricksengine.BrickID {
	return bricksengine.CopyBrickIDs(p.DisableBricks_)
}

// EnableBricks implements Policy.
func (p *policy) EnableBricks() []bricksengine.BrickID {
	return bricksengine.CopyBrickIDs(p.EnableBricks_)
}

// IgnorePreferences implements Policy.
func (p *policy) IgnorePreferences() bool {
	return p.IgnorePreferences_
}

type Policy interface {
	DisableBricks() []bricksengine.BrickID
	EnableBricks() []bricksengine.BrickID
	DisableAuto() bool
	BricksConfigs() map[bricksengine.BrickID]map[string]string
	AllowedMounts() []string
	AllowedProjectRoot() string
	IgnorePreferences() bool
	AllowReverseProxy(port int) bool
}

var defaultPolicy = policy{
	DisableBricks_:      []bricksengine.BrickID{},
	EnableBricks_:       []bricksengine.BrickID{},
	DisableAuto_:        false,
	BricksConfigs_:      map[bricksengine.BrickID]map[string]string{},
	AllowedMounts_:      []string{},
	AllowedProjectRoot_: "",
	IgnorePreferences_:  false,
}

func ensurePolicyLocked(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no policy → fine
		}
		return err
	}

	// Check permission bits only.
	if info.Mode().Perm() != 0o444 {
		return fmt.Errorf("policy file %s must have permissions 0444 (read-only), but has %04o", path, info.Mode().Perm())
	}
	return nil
}

func LoadPolicy() (Policy, error) {
	policyPath, _ := filepath.Abs(hostappconfig.ConfigBasePath() + "policy.json")
	if err := ensurePolicyLocked(policyPath); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(policyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &defaultPolicy, nil
		}
		return nil, err
	}
	var p policy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// HardcodedDeniedPorts is a list of sensitive ports that are ALWAYS blocked
// from reverse proxy access, regardless of policy configuration. This protects
// against malicious containers trying to access sensitive host services.
var HardcodedDeniedPorts = []int{
	// SSH & Remote Login
	22,   // SSH
	2222, // Alternate SSH

	// Remote Desktop
	3389, // RDP (Windows)
	5900, // VNC
	5901, // VNC
	5902, // VNC
	5903, // VNC
	3283, // Apple Remote Desktop
	5800, // VNC over HTTP

	// File Sharing
	445,  // SMB/CIFS
	139,  // NetBIS Session Service
	137,  // NetBIOS Name Service
	138,  // NetBIOS Datagram Service
	2049, // NFS
	111,  // NFS portmapper/rpcbind

	// System Services (potentially dangerous admin interfaces)
	80,   // HTTP (might be admin panel)
	443,  // HTTPS (might be admin panel)
	8080, // Common HTTP alt
	8443, // Common HTTPS alt
	9090, // Common admin panel

	// Container/VM Management
	2375,  // Docker daemon (unencrypted)
	2376,  // Docker daemon (TLS)
	2377,  // Docker Swarm
	8001,  // Kubernetes API proxy
	10250, // Kubelet API
	10255, // Kubelet read-only
	6443,  // Kubernetes API server

	// macOS specific services
	548,  // AFP (Apple File Protocol)
	631,  // CUPS printing
	5353, // mDNS/Bonjour

	// Windows specific
	135,  // Windows RPC
	593,  // Windows RPC over HTTP
	1433, // MS SQL Server (often has weak configs)
	1434, // MS SQL Monitor
	3268, // LDAP Global Catalog
	3269, // LDAP Global Catalog SSL
}

// AllowReverseProxy implements Policy.
// Returns true if the port can be accessed via reverse proxy from the container.
// ALWAYS denies HardcodedDeniedPorts regardless of policy configuration.
func (p *policy) AllowReverseProxy(port int) bool {
	// CRITICAL: Hardcoded ports are ALWAYS denied, no exceptions
	if contains(HardcodedDeniedPorts, port) {
		return false
	}

	// If no reverse proxy policy is configured, allow all non-hardcoded ports
	if p.ReverseProxy_ == nil {
		return true
	}

	// Check custom allowed list (overrides custom denied)
	if len(p.ReverseProxy_.CustomAllowedPorts) > 0 {
		return contains(p.ReverseProxy_.CustomAllowedPorts, port)
	}

	// Check custom denied list
	if len(p.ReverseProxy_.CustomDeniedPorts) > 0 && contains(p.ReverseProxy_.CustomDeniedPorts, port) {
		return false
	}

	// Default: allow
	return true
}

// contains checks if a slice contains a specific integer
func contains(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
