package scaffold

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	docker "github.com/docker/docker/client"
	scaffoldcontainer "github.com/hlfshell/scaffold/container"
)

type ContainerStatus struct {
	ID      string
	Name    string
	Image   string
	State   string
	Status  string
	Labels  map[string]string
	Running bool
}

type NetworkStatus struct {
	ID     string
	Name   string
	Driver string
	Scope  string
	Labels map[string]string
}

type VolumeStatus struct {
	Name       string
	Driver     string
	Mountpoint string
	Labels     map[string]string
}

type ResourceStatus struct {
	Containers []ContainerStatus
	Networks   []NetworkStatus
	Volumes    []VolumeStatus
}

/*
IsRunning returns true if Docker has at least one running container that
matches this stack's labels.
*/
func (s *Stack) IsRunning(ctx context.Context) (bool, error) {
	containers, err := s.RunningContainers(ctx)
	if err != nil {
		return false, err
	}

	return len(containers) > 0, nil
}

/*
RunningContainers returns Docker containers that match this stack's
labels. This is how a defined Go stack can answer whether its matching
local environment is already running.
*/
func (s *Stack) RunningContainers(ctx context.Context) ([]ContainerStatus, error) {
	resources, err := s.Resources(ctx)
	if err != nil {
		return nil, err
	}

	running := []ContainerStatus{}
	for _, container := range resources.Containers {
		if container.Running {
			running = append(running, container)
		}
	}

	return running, nil
}

/*
Resources returns Docker containers, networks, and volumes that match
this stack's labels. This is the broad discovery API for determining
which Docker resources belong to a stack.
*/
func (s *Stack) Resources(ctx context.Context) (ResourceStatus, error) {
	client, err := scaffoldcontainer.NewClient(ctx)
	if err != nil {
		return ResourceStatus{}, err
	}
	defer client.Close()

	filterArgs := s.labelFilters()

	containers, err := client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return ResourceStatus{}, err
	}

	networks, err := client.NetworkList(ctx, network.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return ResourceStatus{}, err
	}

	volumes, err := client.VolumeList(ctx, volume.ListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return ResourceStatus{}, err
	}

	resources := ResourceStatus{
		Containers: []ContainerStatus{},
		Networks:   []NetworkStatus{},
		Volumes:    []VolumeStatus{},
	}

	for _, container := range containers {
		name := ""
		if len(container.Names) > 0 {
			name = container.Names[0]
		}

		resources.Containers = append(resources.Containers, ContainerStatus{
			ID:      container.ID,
			Name:    name,
			Image:   container.Image,
			State:   container.State,
			Status:  container.Status,
			Labels:  cloneLabels(container.Labels),
			Running: container.State == "running",
		})
	}

	for _, network := range networks {
		resources.Networks = append(resources.Networks, NetworkStatus{
			ID:     network.ID,
			Name:   network.Name,
			Driver: network.Driver,
			Scope:  network.Scope,
			Labels: cloneLabels(network.Labels),
		})
	}

	for _, volume := range volumes.Volumes {
		resources.Volumes = append(resources.Volumes, VolumeStatus{
			Name:       volume.Name,
			Driver:     volume.Driver,
			Mountpoint: volume.Mountpoint,
			Labels:     cloneLabels(volume.Labels),
		})
	}

	return resources, nil
}

/*
Down removes Docker resources that match this stack's labels. Unlike
Cleanup, Down does not require this process to have created the stack.
It is intended for CLI and cross-session cleanup.
*/
func (s *Stack) Down(ctx context.Context) error {
	client, err := scaffoldcontainer.NewClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	resources, err := s.Resources(ctx)
	if err != nil {
		return err
	}

	errs := []error{}

	for _, status := range resources.Containers {
		err := removeDiscoveredContainer(ctx, client, status)
		if err != nil {
			errs = append(errs, err)
		}
	}

	for _, status := range resources.Volumes {
		err := client.VolumeRemove(ctx, status.Name, true)
		if err != nil && !docker.IsErrNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to remove volume %s: %w", status.Name, err))
		}
	}

	for _, status := range resources.Networks {
		err := client.NetworkRemove(ctx, status.ID)
		if err != nil && !docker.IsErrNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to remove network %s: %w", status.Name, err))
		}
	}

	s.createdGroups = [][]Service{}
	s.created = false
	s.networkCreated = false

	return errors.Join(errs...)
}

func removeDiscoveredContainer(ctx context.Context, client *docker.Client, status ContainerStatus) error {
	timeout := 10
	err := client.ContainerStop(ctx, status.ID, container.StopOptions{
		Timeout: &timeout,
		Signal:  "SIGTERM",
	})
	if err != nil && !docker.IsErrNotFound(err) {
		if !strings.Contains(err.Error(), "is not running") {
			return fmt.Errorf("failed to stop container %s: %w", status.Name, err)
		}
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		inspect, err := client.ContainerInspect(ctx, status.ID)
		if docker.IsErrNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to inspect container %s: %w", status.Name, err)
		}
		if inspect.State == nil || !inspect.State.Running {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	err = client.ContainerRemove(ctx, status.ID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !docker.IsErrNotFound(err) {
		return fmt.Errorf("failed to remove container %s: %w", status.Name, err)
	}

	return nil
}
