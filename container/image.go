package container

import (
	"context"
	"fmt"
	"io"

	imgtypes "github.com/docker/docker/api/types/image"
	docker "github.com/docker/docker/client"
)

/*
ImageExists will return true if the image/tag exists, false otherwise.
A blank tag is assumed to be "latest".
*/
func (c *Container) ImageExists(ctx context.Context) (bool, error) {
	if err := c.ensureClient(ctx); err != nil {
		return false, err
	}
	return ImageExists(ctx, c.client, c.image, c.tag)
}

/*
DeleteImage removes the container image/tag from the local machine.
*/
func (c *Container) DeleteImage(ctx context.Context) error {
	if err := c.ensureClient(ctx); err != nil {
		return err
	}
	return DeleteImage(ctx, c.client, c.image, c.tag)
}

func (c *Container) pullImage(ctx context.Context) error {
	out, err := c.client.ImagePull(ctx, fmt.Sprintf("%s:%s", c.image, c.tag), imgtypes.PullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(io.Discard, out)
	if err != nil {
		return err
	}

	if exists, err := c.ImageExists(ctx); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("image %s:%s did not successfully pull", c.image, c.tag)
	}

	return nil
}

/*
ImageExists will return true if the image/tag exists, false otherwise.
A blank tag is assumed to be "latest".
*/
func ImageExists(ctx context.Context, client *docker.Client, image string, tag string) (bool, error) {
	if tag == "" {
		tag = "latest"
	}

	_, _, err := client.ImageInspectWithRaw(ctx, fmt.Sprintf("%s:%s", image, tag))
	if err != nil {
		if docker.IsErrNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

/*
DeleteImage removes a specific image/tag from the local machine.
*/
func DeleteImage(ctx context.Context, client *docker.Client, image string, tag string) error {
	exists, err := ImageExists(ctx, client, image, tag)
	if err != nil {
		return fmt.Errorf("failed to check if image exists: %w", err)
	} else if !exists {
		return nil
	}

	_, err = client.ImageRemove(ctx, fmt.Sprintf("%s:%s", image, tag), imgtypes.RemoveOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove image: %w", err)
	}

	return nil
}
