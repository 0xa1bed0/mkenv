package main

import (
	sandbox "github.com/0xa1bed0/mkenv/internal/apps/sandbox/cmds"
	sandboxappconfig "github.com/0xa1bed0/mkenv/internal/apps/sandbox/config"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

// TODO: fix it's start - make cmd option to skip if failed or make config in .mkenv to skip it

func main() {
	var execErr error

	rt := runtime.NewAgentRuntime()
	defer rt.Finalize("mkenv-agent", "Type 'mkenv help' to get help.", &execErr)

	logs.SetFullLogPath(sandboxappconfig.DaemonLogFile)

	execErr = sandbox.Execute(rt)
}
