package dockerclient

import (
	"context"
	"log/slog"
	"os"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/docker/go-sdk/client"
)

type DockerClient struct {
	client client.SDKClient
}

var defaultDockerClient *DockerClient

func DefaultDockerClient() (*DockerClient, error) {
	if defaultDockerClient == nil {
		client, err := client.New(
			context.Background(),
			client.WithLogger(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))),
		)
		if err != nil {
			return nil, err
		}

		defaultDockerClient = &DockerClient{
			client: client,
		}
	}

	return defaultDockerClient, nil
}

func (dc *DockerClient) ImageExists(ctx context.Context, imageRef string) bool {
	logs.Debugf("ImageExists: checking imageRef=%s", imageRef)
	_, err := dc.client.ImageInspect(ctx, imageRef)
	if err != nil {
		logs.Debugf("ImageExists: imageRef=%s not found: %v", imageRef, err)
		return false
	}
	logs.Debugf("ImageExists: imageRef=%s found", imageRef)
	return true
}
