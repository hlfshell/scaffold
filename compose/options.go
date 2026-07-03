package compose

import "time"

/*
Option configures a Compose service.
*/
type Option func(*Compose)

/*
WithFile adds a Compose file from disk.
*/
func WithFile(path string) Option {
	return func(compose *Compose) {
		compose.files = append(compose.files, fileSource{path: path})
	}
}

/*
WithEmbeddedFile adds Compose file contents compiled into the Go binary,
usually from go:embed.
*/
func WithEmbeddedFile(name string, content []byte) Option {
	return func(compose *Compose) {
		compose.files = append(compose.files, fileSource{
			name:    name,
			content: append([]byte(nil), content...),
		})
	}
}

/*
WithProject sets the Docker Compose project name used for commands and
label-based discovery.
*/
func WithProject(project string) Option {
	return func(compose *Compose) {
		compose.project = normalizeProjectName(project)
	}
}

/*
WithWaitTimeout configures the default docker compose --wait timeout.
*/
func WithWaitTimeout(timeout time.Duration) Option {
	return func(compose *Compose) {
		compose.waitTimeout = timeout
	}
}

/*
WithoutWait disables docker compose --wait. Create returns when Compose
finishes creating containers.
*/
func WithoutWait() Option {
	return func(compose *Compose) {
		compose.wait = false
	}
}

/*
WithReadyCheck adds an application-level readiness check after Compose up.
*/
func WithReadyCheck(ready Ready) Option {
	return func(compose *Compose) {
		compose.ready = ready
	}
}

/*
WithBinary overrides the Compose command. The default is docker compose.
*/
func WithBinary(binary string, argsPrefix ...string) Option {
	return func(compose *Compose) {
		compose.binary = binary
		compose.argsPrefix = append([]string(nil), argsPrefix...)
	}
}

func withRunner(runner commandRunner) Option {
	return func(compose *Compose) {
		compose.runner = runner
	}
}
