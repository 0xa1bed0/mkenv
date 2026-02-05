package main

import (
	"os"

	sandbox "github.com/0xa1bed0/mkenv/internal/apps/sandbox/cmds"
	"github.com/0xa1bed0/mkenv/internal/logs"
	sandboxnet "github.com/0xa1bed0/mkenv/internal/networking/sandbox"
	"github.com/0xa1bed0/mkenv/internal/runtime"
)

// TODO: fix it's start - make cmd option to skip if failed or make config in .mkenv to skip it

func main() {
	logs.SetComponent(detectComponent("agent"))

	var execErr error

	rt := runtime.NewAgentRuntime()
	defer rt.Finalize("mkenv-agent", "Type 'mkenv help' to get help.", &execErr)

	if client, err := sandboxnet.NewControlClientFromEnv(rt.Ctx()); err == nil {
		logs.SetFullLogWriter(sandboxnet.NewLogWriter(client))
	}

	execErr = sandbox.Execute(rt)
}

func detectComponent(base string) string {
	if len(os.Args) > 1 && len(os.Args[1]) > 0 && os.Args[1][0] != '-' {
		return base + ":" + os.Args[1]
	}
	return base
}
