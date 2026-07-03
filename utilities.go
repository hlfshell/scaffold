package scaffold

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

/*
Run creates a stack with ctx, runs fn, and then cleans the stack up.
Cleanup is attempted even if fn returns an error.
*/
func Run(ctx context.Context, stack *Stack, fn func(context.Context) error) error {
	err := stack.Create(ctx)
	if err != nil {
		return err
	}

	runErr := fn(ctx)
	cleanupErr := stack.Cleanup(context.WithoutCancel(ctx))
	if runErr != nil && cleanupErr != nil {
		return fmt.Errorf("%w; cleanup failed: %v", runErr, cleanupErr)
	}
	if runErr != nil {
		return runErr
	}

	return cleanupErr
}

/*
Env returns environment variables contributed by services in the stack.
Later services overwrite earlier services if they use the same key.
*/
func (s *Stack) Env() map[string]string {
	env := map[string]string{}
	for _, service := range s.services {
		provider, ok := service.(EnvProvider)
		if !ok {
			continue
		}

		env = mergeLabels(env, provider.Env())
	}

	return env
}

/*
WriteEnvFile writes stack environment variables to a dotenv-style file.
*/
func (s *Stack) WriteEnvFile(path string) error {
	env := s.Env()

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := []string{}
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", key, quoteEnvValue(env[key])))
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

/*
Endpoints returns named endpoints contributed by services in the stack.
*/
func (s *Stack) Endpoints() map[string]string {
	endpoints := map[string]string{}
	for _, service := range s.services {
		provider, ok := service.(EndpointProvider)
		if !ok {
			continue
		}

		for name, endpoint := range provider.Endpoints() {
			endpoints[name] = endpoint
		}
	}

	return endpoints
}

/*
Endpoint returns a named endpoint from the stack.
*/
func (s *Stack) Endpoint(name string) (string, bool) {
	endpoint, ok := s.Endpoints()[name]
	return endpoint, ok
}

/*
Summary returns a human-readable description of the stack service groups
and known endpoints.
*/
func (s *Stack) Summary() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Stack %s\n", s.name))
	if s.networkName != "" {
		builder.WriteString(fmt.Sprintf("Network: %s\n", s.networkName))
	}

	endpoints := s.Endpoints()
	for i, group := range s.serviceGroups {
		builder.WriteString(fmt.Sprintf("\nGroup %d:\n", i+1))
		for _, service := range group {
			builder.WriteString(fmt.Sprintf("  %s", service.Name()))
			if endpoint, ok := endpoints[service.Name()]; ok {
				builder.WriteString(fmt.Sprintf("  %s", endpoint))
			}
			builder.WriteString("\n")
		}
	}

	return strings.TrimRight(builder.String(), "\n")
}

func quoteEnvValue(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\n\"'") {
		return fmt.Sprintf("%q", value)
	}

	return value
}
