package container

import (
	"context"
	"fmt"

	dockercontainer "github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
)

/*
CleanupAndKillContainer kills and removes a container by name, then
removes anonymous volumes that were attached to that container.
*/
func CleanupAndKillContainer(ctx context.Context, client *docker.Client, name string) error {
	volumes := []string{}
	containers, err := client.ContainerList(ctx, dockercontainer.ListOptions{
		All: true,
	})
	if err != nil {
		return err
	}

	containerID := ""
	for _, container := range containers {
		for _, containerName := range container.Names {
			if containerName == fmt.Sprintf("/%s", name) {
				containerID = container.ID
				for _, volume := range container.Mounts {
					if volume.Name != "" {
						volumes = append(volumes, volume.Name)
					}
				}
				break
			}
		}
	}
	if containerID == "" {
		return nil
	}

	err = client.ContainerKill(ctx, containerID, "SIGKILL")
	if err != nil && !docker.IsErrNotFound(err) {
		return err
	}

	err = client.ContainerRemove(ctx, containerID, dockercontainer.RemoveOptions{
		Force: true,
	})
	if err != nil && !docker.IsErrNotFound(err) {
		return err
	}

	for _, volume := range volumes {
		err := client.VolumeRemove(ctx, volume, true)
		if err != nil {
			return err
		}
	}

	return nil
}
