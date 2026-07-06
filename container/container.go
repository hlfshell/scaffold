package container

import (
	"context"
	"fmt"
	"net"
	"sync"

	docker "github.com/docker/docker/client"
)

type Harness interface {
	Start() error
	Stop(wait int) error
	Cleanup() error
	IsRunning() (bool, error)
}

// ContainerOption allows callers to tune a container without making
// the common constructor path noisy.
type ContainerOption func(*Container)

/*
Container is a small wrapper around the Docker API. It owns the Docker
container id, assigned host ports, environment variables, bind mounts,
and anonymous volumes that should be removed during cleanup.
*/
type Container struct {
	client      *docker.Client
	id          string
	name        string
	ports       map[string]string
	env         map[string]string
	image       string
	tag         string
	entrypoint  []string
	command     []string
	volumes     []string
	binds       []string
	networkName string
	labels      map[string]string
	hostIP      string
	privileged  bool

	lock sync.Mutex
}

/*
NewContainer builds a container harness for an image. A blank tag is assumed to
be "latest". If a port mapping has an empty host port, scaffold will assign a
free host port when the container starts.
*/
func NewContainer(name string, image string, options ...ContainerOption) (*Container, error) {
	container := &Container{
		name:   name,
		image:  image,
		tag:    "latest",
		ports:  map[string]string{},
		env:    map[string]string{},
		labels: map[string]string{},
		hostIP: "127.0.0.1",
	}

	for _, option := range options {
		option(container)
	}

	return container, nil
}

/*
Labels returns a copy of the Docker labels configured for the container.
*/
func (c *Container) Labels() map[string]string {
	return cloneLabels(c.labels)
}

/*
Name returns the configured Docker container name.
*/
func (c *Container) Name() string {
	return c.name
}

/*
GetContainerID returns the Docker container id assigned during Start.
*/
func (c *Container) GetContainerID() string {
	return c.id
}

/*
GetPorts returns the container port to host port mapping. Host ports are
available after Start has assigned them.
*/
func (c *Container) GetPorts() map[string]string {
	return cloneStringMap(c.ports)
}

/*
HostPort returns the assigned host port for a container port.
*/
func (c *Container) HostPort(port string) (string, bool) {
	hostPort, ok := c.ports[port]
	if ok {
		return hostPort, true
	}

	normalized := normalizePort(port)
	for key, value := range c.ports {
		if normalizePort(key) == normalized {
			return value, true
		}
	}

	return "", false
}

/*
Address returns localhost:port for a container port. If the port is not
published, it returns an empty string.
*/
func (c *Container) Address(port string) string {
	hostPort, ok := c.HostPort(port)
	if !ok {
		return ""
	}

	host := c.hostIP
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}

	return net.JoinHostPort(host, hostPort)
}

/*
URL returns a local URL for the requested container port.
*/
func (c *Container) URL(scheme string, port string) string {
	address := c.Address(port)
	if address == "" {
		return ""
	}

	return fmt.Sprintf("%s://%s", scheme, address)
}

/*
Client returns the Docker client used by this container.
*/
func (c *Container) Client() *docker.Client {
	return c.client
}

func (c *Container) ensureClient(ctx context.Context) error {
	if c.client != nil {
		return nil
	}

	client, err := NewClient(ctx)
	if err != nil {
		return err
	}

	c.client = client
	return nil
}
