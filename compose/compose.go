package compose

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

const (
	labelProject       = "com.docker.compose.project"
	labelService       = "com.docker.compose.service"
	defaultWaitTimeout = 2 * time.Minute
)

/*
Ready is an optional check that runs after Compose has finished starting the
project. Use it for application-level readiness that is not represented by
Compose healthchecks.
*/
type Ready func(context.Context, *Compose) error

type commandRunner func(context.Context, string, []string, io.Writer, io.Writer) error

/*
Compose wraps a Docker Compose project as a scaffold service.
*/
type Compose struct {
	name        string
	project     string
	files       []fileSource
	wait        bool
	waitTimeout time.Duration
	ready       Ready
	binary      string
	argsPrefix  []string
	runner      commandRunner
}

type fileSource struct {
	path    string
	name    string
	content []byte
}

/*
New creates a Compose-backed scaffold service.
*/
func New(name string, options ...Option) (*Compose, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("compose service requires a name")
	}

	compose := &Compose{
		name:        name,
		project:     normalizeProjectName(name),
		files:       []fileSource{},
		wait:        true,
		waitTimeout: defaultWaitTimeout,
		binary:      "docker",
		argsPrefix:  []string{"compose"},
		runner:      runCommand,
	}

	for _, option := range options {
		option(compose)
	}

	if compose.project == "" {
		return nil, fmt.Errorf("compose project name cannot be empty")
	}
	if len(compose.files) == 0 {
		return nil, fmt.Errorf("compose service requires at least one file")
	}

	return compose, nil
}

func (c *Compose) Name() string {
	return c.name
}

func (c *Compose) Project() string {
	return c.project
}

var invalidProjectCharacter = regexp.MustCompile(`[^a-z0-9_-]+`)

func normalizeProjectName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = invalidProjectCharacter.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-_")
	return name
}
