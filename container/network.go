package container

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
)

/*
CreateNetwork creates a Docker network if it does not already exist.
*/
func CreateNetwork(ctx context.Context, name string, labels map[string]string) (bool, error) {
	client, err := newDockerClient()
	if err != nil {
		return false, err
	}
	defer client.Close()

	_, err = client.NetworkInspect(ctx, name, network.InspectOptions{})
	if err == nil {
		return false, nil
	}
	if err != nil && !docker.IsErrNotFound(err) {
		return false, fmt.Errorf("failed to inspect docker network %s: %w", name, err)
	}

	_, err = client.NetworkCreate(ctx, name, network.CreateOptions{
		Labels: cloneLabels(labels),
	})
	if err != nil {
		return false, fmt.Errorf("failed to create docker network %s: %w", name, err)
	}

	return true, nil
}

/*
RemoveNetwork removes a Docker network. Missing networks are treated as
already cleaned up.
*/
func RemoveNetwork(ctx context.Context, name string) error {
	client, err := newDockerClient()
	if err != nil {
		return err
	}
	defer client.Close()

	err = client.NetworkRemove(ctx, name)
	if err != nil && !docker.IsErrNotFound(err) {
		return fmt.Errorf("failed to remove docker network %s: %w", name, err)
	}

	return nil
}
