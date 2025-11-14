//go:build tools

package registry

//go:generate sh -c "go run ./cmd/mkenv-gen && gofmt -w internal/registry/zz_generated_imports.go"
