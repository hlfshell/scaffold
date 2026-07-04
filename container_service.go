package scaffold

import (
	"context"
	"fmt"
	"time"

	scaffoldcontainer "github.com/hlfshell/scaffold/container"
	"github.com/hlfshell/scaffold/logs"
)

/*
ContainerService is a small service wrapper around Container. It is for
simple services that do not need a toolbox package or custom typed
clients.
*/
type ContainerService struct {
	name      string
	container *scaffoldcontainer.Container
	ready     []func(context.Context, *scaffoldcontainer.Container) error
	endpoints map[string]func(*scaffoldcontainer.Container) string
}

type ContainerServiceOption func(*ContainerService)

/*
FromContainer wraps a plain Docker container as a Service. The service
name defaults to the container name. Use WithName when the container is
unnamed or when the scaffold service should have a different logical
name than the Docker container.
*/
func FromContainer(container *scaffoldcontainer.Container, options ...ContainerServiceOption) (*ContainerService, error) {
	if container == nil {
		return nil, fmt.Errorf("container service requires a container")
	}

	service := &ContainerService{
		name:      container.Name(),
		container: container,
		ready:     []func(context.Context, *scaffoldcontainer.Container) error{},
		endpoints: map[string]func(*scaffoldcontainer.Container) string{},
	}

	for _, option := range options {
		option(service)
	}

	if service.name == "" {
		return nil, fmt.Errorf("container service requires a name: pass a named container or WithName")
	}

	return service, nil
}

/*
WithName sets the scaffold service name for a container-backed service.
It is useful when the Docker container is unnamed or when the service
name should differ from the Docker container name.
*/
func WithName(name string) ContainerServiceOption {
	return func(service *ContainerService) {
		service.name = name
	}
}

/*
WithHTTPReady waits for an HTTP status code on the requested
container port after the container starts.
*/
func WithHTTPReady(port string, path string, statusCode int, timeout time.Duration) ContainerServiceOption {
	return func(service *ContainerService) {
		service.ready = append(service.ready, func(ctx context.Context, container *scaffoldcontainer.Container) error {
			url := container.URL("http", port) + path
			return WaitForHTTP(ctx, url, statusCode, timeout)
		})
	}
}

/*
WithTCPReady waits until the requested container port accepts TCP
connections.
*/
func WithTCPReady(port string, timeout time.Duration) ContainerServiceOption {
	return func(service *ContainerService) {
		service.ready = append(service.ready, func(ctx context.Context, container *scaffoldcontainer.Container) error {
			hostPort, ok := container.HostPort(port)
			if !ok {
				return fmt.Errorf("container did not publish port %s", port)
			}

			return WaitForTCP(ctx, "127.0.0.1", hostPort, timeout)
		})
	}
}

/*
WithEndpoint exposes a named endpoint built from a published container
port.
*/
func WithEndpoint(name string, scheme string, port string) ContainerServiceOption {
	return func(service *ContainerService) {
		service.endpoints[name] = func(container *scaffoldcontainer.Container) string {
			return container.URL(scheme, port)
		}
	}
}

func (s *ContainerService) Name() string {
	return s.name
}

func (s *ContainerService) Create(ctx context.Context) error {
	if err := s.container.Start(ctx); err != nil {
		return err
	}

	for _, ready := range s.ready {
		if err := ready(ctx, s.container); err != nil {
			_ = s.container.Cleanup(context.WithoutCancel(ctx))
			return err
		}
	}

	return nil
}

func (s *ContainerService) Cleanup(ctx context.Context) error {
	return s.container.Cleanup(ctx)
}

func (s *ContainerService) Logs(ctx context.Context) (logs.LogStreams, error) {
	stream, err := s.container.Logs(ctx)
	if err != nil {
		return nil, err
	}

	return logs.LogStreams{s.name: stream}, nil
}

func (s *ContainerService) SetNetwork(name string) {
	s.container.SetNetwork(name)
}

func (s *ContainerService) SetLabels(labels map[string]string) {
	s.container.SetLabels(labels)
}

func (s *ContainerService) SetNamePrefix(prefix string) {
	s.container.SetNamePrefix(prefix)
}

func (s *ContainerService) Endpoints() map[string]string {
	endpoints := map[string]string{}
	for key, value := range s.endpoints {
		endpoints[key] = value(s.container)
	}

	return endpoints
}

func (s *ContainerService) Container() *scaffoldcontainer.Container {
	return s.container
}
