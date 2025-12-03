package main

import (
	mkenv "github.com/0xa1bed0/mkenv/internal/apps/mkenv/cmds"
	_ "github.com/0xa1bed0/mkenv/internal/registry" // blank import triggers init() in internal/bricks/...
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

// TODO: sometimes after "exit" mkenv on host does not exit
// TODO: make additional volumes to the envconfig - do not include them to the signature

func main() {
	var execErr error

	rt := runtime.NewHostRuntime()
	defer rt.Finalize("mkenv", "Type 'mkenv help' to get help.", &execErr)

	execErr = mkenv.Execute(rt)
}
