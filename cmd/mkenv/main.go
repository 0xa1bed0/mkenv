package main

import (
	"github.com/0xa1bed0/mkenv/internal/cli/mkenv"
	_ "github.com/0xa1bed0/mkenv/internal/registry" // blank import triggers init() in internal/bricks/...
)

func main() {
	mkenv.Execute()
}
