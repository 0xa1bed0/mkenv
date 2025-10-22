// Package shells contains brick implementations that configure interactive
// shells within the devcontainer.
package shells

import "github.com/0xa1bed0/mkenv/internal/dockerimage"

// Shell represents a shell that can provide dockerfile patches and additional
// runtime configuration such as .rc snippets.
type Shell interface {
	dockerimage.Brick
	SetRCConfigs(configs []string) error
}
