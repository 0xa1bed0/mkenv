package dockerclient

import (
	"context"
	"log/slog"
	"os"

	"github.com/docker/go-sdk/client"
)

type dockerClient struct {
	client client.SDKClient
}

type DockerClient interface {
	DockerImageBuilder
	DockerContainerRunner
	ImageExists(context.Context, string) bool
}

func NewDockerClient() (*dockerClient, error) {
	client, err := client.New(
		context.Background(),
		client.WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))),
	)
	if err != nil {
		return nil, err
	}

	return &dockerClient{
		client: client,
	}, nil
}

func (dc *dockerClient) ImageExists(ctx context.Context, imageRef string) bool {
	_, err := dc.client.ImageInspect(ctx, imageRef)

	return err == nil
}
