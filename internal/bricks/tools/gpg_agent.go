package tools

import "github.com/0xa1bed0/mkenv/internal/bricksengine"

const gpgAgent = "gpg-agent"

// GPGAgentSocketDir is the container-side directory where host GPG sockets are mounted.
const GPGAgentSocketDir = "/run/user/10000/gnupg"

// GPGHostGnupgMount is where the host's ~/.gnupg is mounted read-only
// so the container can copy private key stubs from private-keys-v1.d/.
const GPGHostGnupgMount = "/home/dev/.gnupg-host"

// GPGExportedPubkeysPath and GPGExportedOwnertrustPath are where the
// host-exported GPG data is mounted. We use gpg --export on the host
// instead of copying keyring files because GPG keyring formats vary
// across versions (kbx in 2.2, keyboxd/SQLite in 2.4+). The exported
// OpenPGP packets are portable across all versions.
const GPGExportedPubkeysPath = "/home/dev/.gnupg-host-export/pubkeys.gpg"
const GPGExportedOwnertrustPath = "/home/dev/.gnupg-host-export/ownertrust.txt"

func NewGPGAgent(metadata map[string]string) (bricksengine.Brick, error) {
	brick, err := bricksengine.NewBrick(gpgAgent, "GPG Agent (forwarded from host)",
		bricksengine.WithKind(bricksengine.BrickKindCommon),
		bricksengine.WithPackageRequest(bricksengine.PackageRequest{
			Reason: "GPG agent forwarding support",
			Packages: []bricksengine.PackageSpec{
				{Name: "gnupg"},
			},
		}),
		// Create the directory for socket mount points
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"mkdir", "-p", GPGAgentSocketDir},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"chown", "${MKENV_USERNAME}:${MKENV_USERNAME}", GPGAgentSocketDir},
		}),
		// Create ~/.gnupg with correct permissions
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"mkdir", "-p", "${MKENV_HOME}/.gnupg"},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"chown", "${MKENV_USERNAME}:${MKENV_USERNAME}", "${MKENV_HOME}/.gnupg"},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"chmod", "700", "${MKENV_HOME}/.gnupg"},
		}),
		// Create mount points for host GNUPG data (bind-mounted read-only at runtime)
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"mkdir", "-p", GPGHostGnupgMount},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"chown", "${MKENV_USERNAME}:${MKENV_USERNAME}", GPGHostGnupgMount},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"mkdir", "-p", "/home/dev/.gnupg-host-export"},
		}),
		bricksengine.WithRootRun(bricksengine.Command{
			When: "build",
			Argv: []string{"chown", "${MKENV_USERNAME}:${MKENV_USERNAME}", "/home/dev/.gnupg-host-export"},
		}),
		bricksengine.WithEnv("SSH_AUTH_SOCK", GPGAgentSocketDir+"/S.gpg-agent.ssh"),
		// Prevent container from starting its own gpg-agent (use forwarded host agent)
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "gpg-conf",
			FilePath: "${MKENV_HOME}/.gnupg/gpg.conf",
			Content:  "no-autostart",
		}),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "gpg-agent",
			FilePath: "rc",
			Content: `# GPG agent forwarding (YubiKey / smartcard)
# updatestartuptty is run on the host before the container starts (see
# orchestrator.go), so pinentry attaches to the correct host TTY.
# Do NOT run it inside the container — the agent needs a host TTY.
export GPG_TTY=$(tty)

# Import host GPG data into container's writable ~/.gnupg.
# Public keys and ownertrust are exported on the host via gpg --export
# (portable across all GPG versions). Key stubs are copied from the
# host's ~/.gnupg (mounted read-only at ~/.gnupg-host).
if [ ! -f "$HOME/.gnupg/.keys-imported" ]; then
  mkdir -p "$HOME/.gnupg/private-keys-v1.d"
  chmod 700 "$HOME/.gnupg" "$HOME/.gnupg/private-keys-v1.d"
  # Import public keys (host-exported OpenPGP packets)
  [ -f "` + GPGExportedPubkeysPath + `" ] && gpg --batch --quiet --import "` + GPGExportedPubkeysPath + `" 2>/dev/null
  # Import ownertrust
  [ -f "` + GPGExportedOwnertrustPath + `" ] && gpg --batch --quiet --import-ownertrust "` + GPGExportedOwnertrustPath + `" 2>/dev/null
  # Copy private key stubs (smartcard references, no secret material)
  if [ -d "$HOME/.gnupg-host/private-keys-v1.d" ]; then
    cp -u "$HOME/.gnupg-host/private-keys-v1.d/"*.key "$HOME/.gnupg/private-keys-v1.d/" 2>/dev/null
  fi
  touch "$HOME/.gnupg/.keys-imported"
fi

# Symlink agent socket so GPG finds the forwarded host agent
if [ -S "` + GPGAgentSocketDir + `/S.gpg-agent" ] && [ ! -L "$HOME/.gnupg/S.gpg-agent" ]; then
  ln -sf "` + GPGAgentSocketDir + `/S.gpg-agent" "$HOME/.gnupg/S.gpg-agent"
fi`,
		}),
	)
	if err != nil {
		return nil, err
	}
	return brick, nil
}

func init() {
	bricksengine.RegisterBrick(gpgAgent, NewGPGAgent)
}
