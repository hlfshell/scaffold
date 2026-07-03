package compose

import (
	"context"
	"strings"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/hlfshell/scaffold"
	"github.com/hlfshell/scaffold/container"
	"github.com/hlfshell/scaffold/logs"
)

/*
Logs returns one Docker log stream per discovered Compose service
container.
*/
func (c *Compose) Logs(ctx context.Context) (logs.LogStreams, error) {
	client, err := container.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	containers, err := client.ContainerList(ctx, dockercontainer.ListOptions{
		All:     true,
		Filters: c.filters(),
	})
	if err != nil {
		return nil, err
	}

	streams := logs.LogStreams{}
	for _, container := range containers {
		stream, err := client.ContainerLogs(ctx, container.ID, dockercontainer.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
		})
		if err != nil {
			_ = streams.Close()
			return nil, err
		}

		name := container.Labels[labelService]
		if name == "" {
			name = strings.TrimPrefix(firstContainerName(container.Names), "/")
		}
		if name == "" {
			name = container.ID[:12]
		}

		streams[logs.UniqueName(streams, name)] = stream
	}

	return streams, nil
}

/*
IsRunning returns true when any discovered Compose container is running.
*/
func (c *Compose) IsRunning(ctx context.Context) (bool, error) {
	resources, err := c.Resources(ctx)
	if err != nil {
		return false, err
	}

	for _, container := range resources.Containers {
		if container.Running {
			return true, nil
		}
	}

	return false, nil
}

/*
Resources discovers containers, networks, and volumes by Compose project
label. This works across process restarts as long as the project name is
stable.
*/
func (c *Compose) Resources(ctx context.Context) (scaffold.ResourceStatus, error) {
	client, err := container.NewClient(ctx)
	if err != nil {
		return scaffold.ResourceStatus{}, err
	}
	defer client.Close()

	containers, err := client.ContainerList(ctx, dockercontainer.ListOptions{
		All:     true,
		Filters: c.filters(),
	})
	if err != nil {
		return scaffold.ResourceStatus{}, err
	}

	networks, err := client.NetworkList(ctx, network.ListOptions{
		Filters: c.filters(),
	})
	if err != nil {
		return scaffold.ResourceStatus{}, err
	}

	volumes, err := client.VolumeList(ctx, volume.ListOptions{
		Filters: c.filters(),
	})
	if err != nil {
		return scaffold.ResourceStatus{}, err
	}

	resources := scaffold.ResourceStatus{
		Containers: []scaffold.ContainerStatus{},
		Networks:   []scaffold.NetworkStatus{},
		Volumes:    []scaffold.VolumeStatus{},
	}

	for _, container := range containers {
		resources.Containers = append(resources.Containers, scaffold.ContainerStatus{
			ID:      container.ID,
			Name:    firstContainerName(container.Names),
			Image:   container.Image,
			State:   container.State,
			Status:  container.Status,
			Labels:  cloneLabels(container.Labels),
			Running: container.State == "running",
		})
	}

	for _, network := range networks {
		resources.Networks = append(resources.Networks, scaffold.NetworkStatus{
			ID:     network.ID,
			Name:   network.Name,
			Driver: network.Driver,
			Scope:  network.Scope,
			Labels: cloneLabels(network.Labels),
		})
	}

	for _, volume := range volumes.Volumes {
		resources.Volumes = append(resources.Volumes, scaffold.VolumeStatus{
			Name:       volume.Name,
			Driver:     volume.Driver,
			Mountpoint: volume.Mountpoint,
			Labels:     cloneLabels(volume.Labels),
		})
	}

	return resources, nil
}

func (c *Compose) filters() filters.Args {
	args := filters.NewArgs()
	args.Add("label", labelProject+"="+c.project)
	return args
}

func firstContainerName(names []string) string {
	if len(names) == 0 {
		return ""
	}

	return names[0]
}

func cloneLabels(labels map[string]string) map[string]string {
	cloned := map[string]string{}
	for key, value := range labels {
		cloned[key] = value
	}

	return cloned
}
