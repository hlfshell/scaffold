package container

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/hlfshell/scaffold/logs"
)

/*
Logs returns a Docker log reader for the container.
*/
func (c *Container) Logs(ctx context.Context) (io.ReadCloser, error) {
	if err := c.ensureClient(ctx); err != nil {
		return nil, err
	}
	if c.id == "" {
		return nil, fmt.Errorf("container has not been started")
	}

	return c.client.ContainerLogs(ctx, c.id, dockercontainer.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
}

/*
WaitForLogText waits for text to appear in the container logs.
*/
func (c *Container) WaitForLogText(ctx context.Context, text string, timeout time.Duration) error {
	logs, err := c.Logs(ctx)
	if err != nil {
		return err
	}
	defer logs.Close()

	return waitForLogText(ctx, logs, text, timeout)
}

func waitForLogText(ctx context.Context, reader io.Reader, text string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), text) {
				done <- nil
				return
			}
		}
		if err := scanner.Err(); err != nil {
			done <- err
			return
		}
		done <- io.EOF
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for log text %q", text)
	}
}

/*
LogsFromContainers returns one named log stream for each container. Named
containers use their Docker name. Unnamed containers receive a stable
container-N key based on argument order.
*/
func LogsFromContainers(ctx context.Context, containers ...*Container) (logs.LogStreams, error) {
	streams := logs.LogStreams{}
	for i, container := range containers {
		if container == nil {
			_ = streams.Close()
			return nil, fmt.Errorf("container %d is nil", i)
		}

		name := container.Name()
		if name == "" {
			name = fmt.Sprintf("container-%d", i+1)
		}

		stream, err := container.Logs(ctx)
		if err != nil {
			_ = streams.Close()
			return nil, fmt.Errorf("failed to open logs for %s: %w", name, err)
		}

		streams[logs.UniqueName(streams, name)] = stream
	}

	return streams, nil
}

/*
MergeContainerLogs opens logs for N containers and merges them into one reader.
*/
func MergeContainerLogs(ctx context.Context, containers ...*Container) (io.ReadCloser, error) {
	streams, err := LogsFromContainers(ctx, containers...)
	if err != nil {
		return nil, err
	}

	return logs.Merge(streams), nil
}
