package container

import (
	"context"
	"fmt"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

/*
IsRunning returns true if the container is running, false otherwise.
*/
func (c *Container) IsRunning(ctx context.Context) (bool, error) {
	if err := c.ensureClient(ctx); err != nil {
		return false, err
	}
	if c.id == "" {
		return false, nil
	}

	container, err := c.client.ContainerInspect(ctx, c.id)
	if err != nil && docker.IsErrNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return container.State.Running, nil
}

/*
Start pulls the image if needed and starts the container. If port
mappings are configured with an empty host port, a free port is assigned
before the container is created.
*/
func (c *Container) Start(ctx context.Context) (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.ensureClient(ctx); err != nil {
		return err
	}

	if running, err := c.IsRunning(ctx); err != nil {
		return err
	} else if running {
		return nil
	}

	// If a scaffold-owned container with the same name already exists,
	// remove it so repeated tests and local development runs start from a
	// clean state. Non-scaffold containers are left alone.
	if c.name != "" {
		containers, err := c.client.ContainerList(ctx, dockercontainer.ListOptions{
			All: true,
		})
		if err != nil {
			return err
		}

		for _, existing := range containers {
			for _, name := range existing.Names {
				if name == fmt.Sprintf("/%s", c.name) {
					if !isScaffoldOwned(existing.Labels) {
						return fmt.Errorf("container name %s already exists and is not owned by scaffold", c.name)
					}

					err = CleanupAndKillContainer(ctx, c.client, c.name)
					if err != nil {
						return err
					}
					break
				}
			}
		}
	}

	// Attempt to pull the container if we do not have the image locally.
	if exists, err := c.ImageExists(ctx); err != nil {
		return err
	} else if !exists {
		if err := c.pullImage(ctx); err != nil {
			return err
		}
	}

	// Convert the env map to Docker's KEY=VALUE format.
	env := []string{}
	for k, v := range c.env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create the exposed ports and host port bindings Docker expects.
	exposedPorts := nat.PortSet{}
	for k := range c.ports {
		port := normalizePort(k)
		exposedPorts[nat.Port(port)] = struct{}{}
	}

	portBindings := nat.PortMap{}
	for k, v := range c.ports {
		port := normalizePort(k)

		portBindings[nat.Port(port)] = []nat.PortBinding{
			{
				HostIP:   c.hostIP,
				HostPort: v,
			},
		}
	}

	containerConfig := &dockercontainer.Config{
		Image:        fmt.Sprintf("%s:%s", c.image, c.tag),
		Cmd:          c.command,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels:       cloneLabels(c.labels),
	}
	hostConfig := &dockercontainer.HostConfig{
		Binds:        c.binds,
		PortBindings: portBindings,
	}

	networkConfig := &network.NetworkingConfig{}
	if c.networkName != "" {
		networkConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			c.networkName: {},
		}
	}

	response, err := c.client.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		networkConfig,
		nil,
		c.name,
	)
	if err != nil {
		return err
	}
	c.id = response.ID
	createdID := response.ID
	defer func() {
		if err != nil && createdID != "" {
			_ = c.client.ContainerRemove(context.WithoutCancel(ctx), createdID, dockercontainer.RemoveOptions{
				Force:         true,
				RemoveVolumes: true,
			})
			c.id = ""
		}
	}()

	err = c.client.ContainerStart(ctx, response.ID, dockercontainer.StartOptions{})
	if err != nil {
		return err
	}

	inspect, err := c.client.ContainerInspect(ctx, response.ID)
	if err != nil {
		return err
	}

	for key := range c.ports {
		port := nat.Port(normalizePort(key))
		bindings := inspect.NetworkSettings.Ports[port]
		if len(bindings) > 0 {
			c.ports[key] = bindings[0].HostPort
		}
	}

	// Identify anonymous volumes attached to the container so Cleanup can
	// remove them later.
	volumes := []string{}
	for _, mount := range inspect.Mounts {
		if mount.Name != "" {
			volumes = append(volumes, mount.Name)
		}
	}
	c.volumes = volumes

	return nil
}

/*
Stop sends SIGTERM and waits up to the given number of seconds. If the
container does not stop in time, or wait is <= 0, it is killed with
SIGKILL.
*/
func (c *Container) Stop(ctx context.Context, wait int) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if running, err := c.IsRunning(ctx); err != nil {
		return err
	} else if !running {
		return nil
	}

	if wait > 0 {
		startedAt := time.Now()
		err := c.client.ContainerStop(ctx, c.id, dockercontainer.StopOptions{
			Timeout: &wait,
			Signal:  "SIGTERM",
		})
		if err != nil {
			duration := time.Since(startedAt)
			if duration < time.Duration(wait)*time.Second {
				return err
			}
		}

		if running, err := c.IsRunning(ctx); err != nil {
			return err
		} else if !running {
			return nil
		}
	}

	timeout := -1
	err := c.client.ContainerStop(ctx, c.id, dockercontainer.StopOptions{
		Timeout: &timeout,
		Signal:  "SIGKILL",
	})
	if err != nil {
		return err
	}

	if running, err := c.IsRunning(ctx); err != nil {
		return err
	} else if !running {
		return nil
	}

	return fmt.Errorf("container %s did not stop", c.id)
}

/*
Kill immediately terminates the container without waiting for graceful
shutdown inside the container.
*/
func (c *Container) Kill(ctx context.Context) error {
	return c.Stop(ctx, -1)
}

/*
Cleanup kills the container if it is still running, removes it, and then
removes anonymous volumes that were attached to it.
*/
func (c *Container) Cleanup(ctx context.Context) error {
	if err := c.ensureClient(ctx); err != nil {
		return err
	}
	if c.id == "" {
		return nil
	}

	if running, err := c.IsRunning(ctx); err != nil {
		return err
	} else if running {
		err := c.Kill(ctx)
		if err != nil {
			return err
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	err := c.client.ContainerRemove(ctx, c.id, dockercontainer.RemoveOptions{})
	if err != nil && !docker.IsErrNotFound(err) {
		return err
	}

	for _, volume := range c.volumes {
		err := c.client.VolumeRemove(ctx, volume, true)
		if err != nil {
			return err
		}
	}

	c.id = ""
	return nil
}
