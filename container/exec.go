package container

import (
	"bytes"
	"context"
	"fmt"
	"io"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

/*
Exec runs a command inside the started container and returns combined
stdout and stderr output.
*/
func (c *Container) Exec(ctx context.Context, command ...string) ([]byte, error) {
	return c.ExecInput(ctx, nil, command...)
}

/*
ExecInput runs a command inside the started container, writes input to
stdin, and returns combined stdout and stderr output.
*/
func (c *Container) ExecInput(ctx context.Context, input io.Reader, command ...string) ([]byte, error) {
	if err := c.ensureClient(ctx); err != nil {
		return nil, err
	}
	if c.id == "" {
		return nil, fmt.Errorf("container has not been started")
	}

	created, err := c.client.ContainerExecCreate(ctx, c.id, dockercontainer.ExecOptions{
		Cmd:          command,
		AttachStdin:  input != nil,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, err
	}

	attached, err := c.client.ContainerExecAttach(ctx, created.ID, dockercontainer.ExecAttachOptions{})
	if err != nil {
		return nil, err
	}
	defer attached.Close()

	output := bytes.Buffer{}
	copyDone := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(&output, &output, attached.Reader)
		copyDone <- err
	}()

	if input != nil {
		if _, err := io.Copy(attached.Conn, input); err != nil {
			return output.Bytes(), err
		}
		if err := attached.CloseWrite(); err != nil {
			return output.Bytes(), err
		}
	}

	copyErr := <-copyDone

	inspect, err := c.client.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return output.Bytes(), err
	}
	if copyErr != nil {
		return output.Bytes(), copyErr
	}
	if inspect.ExitCode != 0 {
		return output.Bytes(), fmt.Errorf("container exec exited with code %d", inspect.ExitCode)
	}

	return output.Bytes(), nil
}
