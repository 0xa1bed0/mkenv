package dockerclient

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/version"
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

// GetImageSchemaVersion returns the schema version of an image.
// Returns -1 if the image has no schema label (legacy image) or if there's an error.
func (dc *DockerClient) GetImageSchemaVersion(ctx context.Context, imageRef string) int {
	imageInspect, err := dc.client.ImageInspect(ctx, imageRef)
	if err != nil {
		return -1
	}
	schemaStr, ok := imageInspect.Config.Labels[version.ImageSchemaVersionLabel]
	if !ok {
		return -1 // legacy image without schema label
	}
	imageSchema, err := strconv.Atoi(schemaStr)
	if err != nil {
		return -1
	}
	return imageSchema
}

func (dc *DockerClient) IsImageSchemaCompatible(ctx context.Context, imageRef string) bool {
	return dc.GetImageSchemaVersion(ctx, imageRef) == version.ImageSchemaVersion
}
