package container

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	docker "github.com/docker/docker/client"
)

const labelManagedBy = "scaffold.managed-by"

/*
DockerAvailable returns true when a Docker daemon is reachable from the
current environment.
*/
func DockerAvailable() bool {
	client, err := newDockerClient()
	if err != nil {
		return false
	}
	defer client.Close()

	_, err = client.Ping(context.Background())
	return err == nil
}

func normalizePort(port string) string {
	if !strings.Contains(port, "/") {
		return fmt.Sprintf("%s/tcp", port)
	}
	return port
}

func cloneStringMap(values map[string]string) map[string]string {
	cloned := map[string]string{}
	for key, value := range values {
		cloned[key] = value
	}

	return cloned
}

func cloneLabels(labels map[string]string) map[string]string {
	return cloneStringMap(labels)
}

func mergeLabels(base map[string]string, extra map[string]string) map[string]string {
	merged := cloneStringMap(base)
	for key, value := range extra {
		merged[key] = value
	}

	return merged
}

func isScaffoldOwned(labels map[string]string) bool {
	return labels[labelManagedBy] == "scaffold"
}

func NewClient(ctx context.Context) (*docker.Client, error) {
	if os.Getenv("DOCKER_HOST") != "" {
		return pingClient(ctx, docker.FromEnv)
	}

	candidates := []string{
		"unix:///var/run/docker.sock",
	}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		candidates = append(candidates, "unix://"+filepath.Join(runtimeDir, "podman", "podman.sock"))
	}
	candidates = append(candidates,
		fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", os.Getuid()),
		"unix:///var/run/podman/podman.sock",
	)

	var lastErr error
	for _, host := range candidates {
		client, err := pingClient(ctx, docker.WithHost(host))
		if err == nil {
			return client, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to connect to Docker or Podman API: %w", lastErr)
}

func newDockerClient() (*docker.Client, error) {
	return NewClient(context.Background())
}

func pingClient(ctx context.Context, hostOpt docker.Opt) (*docker.Client, error) {
	client, err := docker.NewClientWithOpts(hostOpt, docker.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	_, err = client.Ping(ctx)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}
