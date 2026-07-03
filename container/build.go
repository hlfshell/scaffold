package container

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/build"
)

/*
BuildDockerfile builds the Dockerfile at dockerfilePath. The build
context is the Dockerfile's directory. It returns the image id Docker
reported, the combined Docker build logs, and any error.
*/
func BuildDockerfile(ctx context.Context, dockerfilePath string) (string, string, error) {
	contextReader, dockerfileName, err := dockerBuildContext(dockerfilePath)
	if err != nil {
		return "", "", err
	}

	buildOptions := build.ImageBuildOptions{
		Dockerfile: dockerfileName,
		Remove:     true,
	}

	client, err := newDockerClient()
	if err != nil {
		return "", "", err
	}
	defer client.Close()

	response, err := client.ImageBuild(ctx, contextReader, buildOptions)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()

	image, logs, err := readDockerBuildOutput(response.Body)
	if err != nil {
		return image, logs, err
	}

	return image, logs, nil
}

func dockerBuildContext(dockerfilePath string) (io.Reader, string, error) {
	if dockerfilePath == "" {
		return nil, "", fmt.Errorf("dockerfile path is required")
	}

	absoluteDockerfile, err := filepath.Abs(dockerfilePath)
	if err != nil {
		return nil, "", err
	}

	info, err := os.Stat(absoluteDockerfile)
	if err != nil {
		return nil, "", err
	}
	if info.IsDir() {
		return nil, "", fmt.Errorf("dockerfile path %s is a directory", dockerfilePath)
	}

	contextDir := filepath.Dir(absoluteDockerfile)
	buffer := &bytes.Buffer{}
	writer := tar.NewWriter(buffer)

	err = filepath.WalkDir(contextDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(contextDir, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)

		if err := writer.WriteHeader(header); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(writer, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}

		return closeErr
	})
	if err != nil {
		_ = writer.Close()
		return nil, "", err
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return buffer, filepath.Base(absoluteDockerfile), nil
}

type dockerBuildMessage struct {
	Stream      string `json:"stream"`
	Status      string `json:"status"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
	Aux struct {
		ID string `json:"ID"`
	} `json:"aux"`
}

func readDockerBuildOutput(reader io.Reader) (string, string, error) {
	image := ""
	var logs strings.Builder

	decoder := json.NewDecoder(reader)
	for {
		var message dockerBuildMessage
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}

			remainder, _ := io.ReadAll(reader)
			logs.Write(remainder)
			return image, logs.String(), fmt.Errorf("failed to decode docker build output: %w", err)
		}

		if message.Stream != "" {
			logs.WriteString(message.Stream)
		}
		if message.Status != "" {
			logs.WriteString(message.Status)
			logs.WriteString("\n")
		}
		if message.Aux.ID != "" {
			image = message.Aux.ID
		}
		if message.Error != "" {
			logs.WriteString(message.Error)
			logs.WriteString("\n")
			return image, logs.String(), fmt.Errorf("docker build failed: %s", message.Error)
		}
		if message.ErrorDetail.Message != "" {
			logs.WriteString(message.ErrorDetail.Message)
			logs.WriteString("\n")
			return image, logs.String(), fmt.Errorf("docker build failed: %s", message.ErrorDetail.Message)
		}
	}

	return image, logs.String(), nil
}
