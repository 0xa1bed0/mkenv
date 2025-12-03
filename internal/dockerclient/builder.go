package dockerclient

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/go-sdk/client"
	sdkimage "github.com/docker/go-sdk/image"

	"github.com/0xa1bed0/mkenv/internal/logs"
)

func (dc *DockerClient) BuildImage(ctx context.Context, dockerfile string, tag string) (string, error) {
	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)

	dockerFileBytes := []byte(dockerfile)
	tarHeader := &tar.Header{
		Name: "Dockerfile",
		Mode: 0o600,
		Size: int64(len(dockerFileBytes)),
	}

	if err := tarWriter.WriteHeader(tarHeader); err != nil {
		return "", fmt.Errorf("write tar header: %w", err)
	}
	if _, err := tarWriter.Write(dockerFileBytes); err != nil {
		return "", fmt.Errorf("write dockerfile: %w", err)
	}
	if err := tarWriter.Close(); err != nil {
		return "", fmt.Errorf("close tar: %w", err)
	}

	tailbox := logs.NewTailBox("docker build")
	client, err := client.New(
		context.Background(),
		client.WithLogger(slog.New(slog.NewTextHandler(tailbox, &slog.HandlerOptions{}))),
	)
	if err != nil {
		return "", fmt.Errorf("get docker client for build: %v", err)
	}

	buildTag, err := sdkimage.Build(
		ctx,
		&buf,
		tag,
		sdkimage.WithBuildClient(client),
		sdkimage.WithBuildOptions(build.ImageBuildOptions{
			Dockerfile: "Dockerfile",
			Remove:     true, // remove intermediate containers
		}),
	)
	tailbox.Close()
	if err != nil {
		return "", fmt.Errorf("image build: %w", err)
	}

	return buildTag, nil
}
