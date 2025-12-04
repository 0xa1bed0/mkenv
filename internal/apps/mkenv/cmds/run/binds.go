package runcmd

import (
	"strings"

	sandboxappconfig "github.com/0xa1bed0/mkenv/internal/apps/sandbox/config"
	"github.com/0xa1bed0/mkenv/internal/utils"
)

func ResolveBinds(binds []string) ([]string, error) {
	var err error
	out := []string{}

	for _, bind := range binds {
		bindConf := strings.Split(bind, ":")
		hostPath := bindConf[0]
		containerPath := bindConf[1]
		perm := "rw"
		if len(bindConf) >= 3 {
			perm = bindConf[2]
		}

		hostPath, err = utils.ResolvePathStrict(hostPath)
		if err != nil {
			return nil, err
		}
		containerPath = strings.Replace(containerPath, "~", sandboxappconfig.HomeFolder, 1)
		out = append(out, strings.Join([]string{hostPath, containerPath, perm}, ":"))
	}

	return out, nil
}
