package compose

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (c *Compose) run(ctx context.Context, stdout io.Writer, stderr io.Writer, args ...string) error {
	files, cleanup, err := c.composeFiles()
	if err != nil {
		return err
	}
	defer cleanup()

	fullArgs := append([]string{}, c.argsPrefix...)
	fullArgs = append(fullArgs, "-p", c.project)
	for _, file := range files {
		fullArgs = append(fullArgs, "-f", file)
	}
	fullArgs = append(fullArgs, args...)

	return c.runner(ctx, c.binary, fullArgs, stdout, stderr)
}

func (c *Compose) composeFiles() ([]string, func(), error) {
	files := make([]string, 0, len(c.files))
	tempDirs := []string{}

	cleanup := func() {
		for _, dir := range tempDirs {
			_ = os.RemoveAll(dir)
		}
	}

	for i, file := range c.files {
		if file.path != "" {
			files = append(files, file.path)
			continue
		}

		dir, err := os.MkdirTemp("", "scaffold-compose-*")
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		tempDirs = append(tempDirs, dir)

		name := file.name
		if name == "" {
			name = fmt.Sprintf("compose-%d.yml", i+1)
		}
		path := filepath.Join(dir, filepath.Base(name))
		if err := os.WriteFile(path, file.content, 0600); err != nil {
			cleanup()
			return nil, nil, err
		}

		files = append(files, path)
	}

	return files, cleanup, nil
}

func runCommand(ctx context.Context, binary string, args []string, stdout io.Writer, stderr io.Writer) error {
	command := exec.CommandContext(ctx, binary, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func commandError(message string, err error, stdout *bytes.Buffer, stderr *bytes.Buffer) error {
	output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	if output == "" {
		return fmt.Errorf("%s: %w", message, err)
	}

	return fmt.Errorf("%s: %w\n%s", message, err, output)
}
