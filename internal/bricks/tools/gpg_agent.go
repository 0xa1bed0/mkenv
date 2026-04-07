package tools

import "github.com/0xa1bed0/mkenv/internal/bricksengine"

const gpgAgent = "gpg-agent"

// GPGAgentSocketDir is the container-side directory where host GPG sockets are mounted.
const GPGAgentSocketDir = "/run/user/10000/gnupg"

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
		bricksengine.WithEnv("SSH_AUTH_SOCK", GPGAgentSocketDir+"/S.gpg-agent.ssh"),
		bricksengine.WithFileTemplate(bricksengine.FileTemplate{
			ID:       "gpg-agent",
			FilePath: "rc",
			Content: `# GPG agent forwarding (YubiKey / smartcard)
# Do NOT run updatestartuptty here — the agent runs on the host and needs
# a host TTY for pinentry. Setting a container PTY would break PIN prompts.
export GPG_TTY=$(tty)`,
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
