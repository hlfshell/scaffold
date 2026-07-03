package compose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
)

/*
Create starts the Compose project. If Compose fails after creating partial
resources, Create attempts to clean them up before returning the error.
*/
func (c *Compose) Create(ctx context.Context) error {
	output := &bytes.Buffer{}
	errOutput := &bytes.Buffer{}
	err := c.run(ctx, output, errOutput, c.upArgs()...)
	if err != nil {
		resources, discoverErr := c.Resources(ctx)
		_ = c.Cleanup(context.WithoutCancel(ctx))
		if discoverErr != nil {
			return errors.Join(commandError("compose up failed", err, output, errOutput), discoverErr)
		}

		return fmt.Errorf(
			"compose up failed after creating %d containers, %d networks, and %d volumes: %w",
			len(resources.Containers),
			len(resources.Networks),
			len(resources.Volumes),
			commandError("compose up failed", err, output, errOutput),
		)
	}

	if c.ready != nil {
		if err := c.ready(ctx, c); err != nil {
			_ = c.Cleanup(context.WithoutCancel(ctx))
			return err
		}
	}

	return nil
}

/*
Cleanup runs docker compose down for the configured project.
*/
func (c *Compose) Cleanup(ctx context.Context) error {
	return c.Down(ctx)
}

/*
Down removes resources for this Compose project, including resources
created by an earlier process using the same project name.
*/
func (c *Compose) Down(ctx context.Context) error {
	output := &bytes.Buffer{}
	errOutput := &bytes.Buffer{}
	err := c.run(ctx, output, errOutput, "down", "--remove-orphans", "--volumes")
	if err != nil {
		return commandError("compose down failed", err, output, errOutput)
	}

	return nil
}

func (c *Compose) upArgs() []string {
	args := []string{"up", "-d"}
	if c.wait {
		args = append(args, "--wait")
		if c.waitTimeout > 0 {
			args = append(args, "--wait-timeout", fmt.Sprintf("%.0f", c.waitTimeout.Seconds()))
		}
	}

	return args
}
