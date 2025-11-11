package dockerclient

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types/build"
	sdkimage "github.com/docker/go-sdk/image"
)

type DockerImageBuilder interface {
	BuildImage(ctx context.Context, dockerfile string, tag string) (string, error)
}

func (dc *dockerClient) BuildImage(ctx context.Context, dockerfile string, tag string) (string, error) {
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

	buildTag, err := sdkimage.Build(
		ctx,
		&buf,
		tag,
		sdkimage.WithBuildClient(dc.client),
		sdkimage.WithBuildOptions(build.ImageBuildOptions{
			Dockerfile: "Dockerfile",
			Remove:     true, // remove intermediate containers
		}),
	)
	if err != nil {
		return "", fmt.Errorf("image build: %w", err)
	}

	return buildTag, nil
}
