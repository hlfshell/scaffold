package container

import (
	"fmt"
	"strings"
)

/*
WithTag sets the image tag used when the container starts. A blank tag
keeps the default tag, "latest".
*/
func WithTag(tag string) ContainerOption {
	return func(c *Container) {
		if tag == "" {
			return
		}

		c.tag = tag
	}
}

/*
WithPorts adds container port to host port mappings. An empty host port
asks scaffold to assign a free host port when the container starts.
*/
func WithPorts(ports map[string]string) ContainerOption {
	return func(c *Container) {
		for containerPort, hostPort := range ports {
			c.ports[containerPort] = hostPort
		}
	}
}

/*
WithPort adds one container port to host port mapping. An empty host port
asks scaffold to assign a free host port when the container starts.
*/
func WithPort(containerPort string, hostPort string) ContainerOption {
	return func(c *Container) {
		c.ports[containerPort] = hostPort
	}
}

/*
WithEnv adds container process environment variables. These values are
sent to Docker as KEY=VALUE entries when the container starts.
*/
func WithEnv(env map[string]string) ContainerOption {
	return func(c *Container) {
		for key, value := range env {
			c.env[key] = value
		}
	}
}

/*
WithLabels adds Docker labels to the container. These labels are merged
with any labels inherited from a parent stack.
*/
func WithLabels(labels map[string]string) ContainerOption {
	return func(c *Container) {
		c.labels = mergeLabels(c.labels, labels)
	}
}

/*
WithHostIP sets the host IP used for published container ports. The
default is 127.0.0.1 for local-only services.
*/
func WithHostIP(hostIP string) ContainerOption {
	return func(c *Container) {
		c.hostIP = hostIP
	}
}

/*
WithBind mounts a host path into the container. This is intentionally
simple and maps directly to Docker's bind syntax.
*/
func WithBind(hostPath string, containerPath string) ContainerOption {
	return func(c *Container) {
		c.binds = append(c.binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}
}

/*
WithNetwork attaches the container to an existing Docker network when it
is created.
*/
func WithNetwork(name string) ContainerOption {
	return func(c *Container) {
		c.networkName = name
	}
}

/*
WithCommand sets the container command. This is useful for images like
MinIO that require a command to start the desired service.
*/
func WithCommand(command ...string) ContainerOption {
	return func(c *Container) {
		c.command = command
	}
}

/*
SetNetwork updates the Docker network name for the container. Stacks use
this to attach services to a shared network before creation.
*/
func (c *Container) SetNetwork(name string) {
	c.networkName = name
}

/*
SetLabels merges Docker labels onto the container.
*/
func (c *Container) SetLabels(labels map[string]string) {
	c.labels = mergeLabels(c.labels, labels)
}

/*
SetNamePrefix prefixes the Docker container name if one was configured.
Stacks call this before creation for services that opt into generated
resource names.
*/
func (c *Container) SetNamePrefix(prefix string) {
	if c.name == "" || prefix == "" {
		return
	}
	if strings.HasPrefix(c.name, prefix+"-") {
		return
	}

	c.name = fmt.Sprintf("%s-%s", prefix, c.name)
}
