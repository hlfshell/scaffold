package container

import (
	"context"
	"strings"
	"testing"
)

func TestContainerExecRequiresStartedContainer(t *testing.T) {
	ctx := context.Background()
	container, err := NewContainer("scaffold-test-exec-not-started", "alpine", WithTag("latest"))
	if err != nil {
		t.Fatal(err)
	}

	output, err := container.Exec(ctx, "sh", "-c", "echo should-not-run")
	if err == nil {
		t.Fatal("expected exec on a stopped container to fail")
	}
	if output != nil {
		t.Fatalf("expected nil output before exec starts, got %q", string(output))
	}
	if !strings.Contains(err.Error(), "container has not been started") {
		t.Fatalf("expected not started error, got %v", err)
	}
}

func TestContainerExecCapturesStdoutAndStderr(t *testing.T) {
	ctx := context.Background()
	container := startExecTestContainer(t, ctx, "scaffold-test-exec-output")

	output, err := container.Exec(ctx, "sh", "-c", "echo stdout; echo stderr >&2")
	if err != nil {
		t.Fatal(err)
	}

	text := string(output)
	if !strings.Contains(text, "stdout") {
		t.Fatalf("expected stdout in exec output, got %q", text)
	}
	if !strings.Contains(text, "stderr") {
		t.Fatalf("expected stderr in exec output, got %q", text)
	}
}

func TestContainerExecReturnsOutputOnNonZeroExit(t *testing.T) {
	ctx := context.Background()
	container := startExecTestContainer(t, ctx, "scaffold-test-exec-failure")

	output, err := container.Exec(ctx, "sh", "-c", "echo before-fail; exit 7")
	if err == nil {
		t.Fatal("expected non-zero exec to fail")
	}
	if !strings.Contains(err.Error(), "exited with code 7") {
		t.Fatalf("expected exit code in error, got %v", err)
	}
	if !strings.Contains(string(output), "before-fail") {
		t.Fatalf("expected output from failing exec, got %q", string(output))
	}
}

func TestContainerExecInputWritesStdin(t *testing.T) {
	ctx := context.Background()
	container := startExecTestContainer(t, ctx, "scaffold-test-exec-input")

	output, err := container.ExecInput(ctx, strings.NewReader("hello from stdin"), "cat")
	if err != nil {
		t.Fatal(err)
	}
	if string(output) != "hello from stdin" {
		t.Fatalf("expected stdin to be echoed, got %q", string(output))
	}
}

func startExecTestContainer(t *testing.T, ctx context.Context, name string) *Container {
	t.Helper()

	if !DockerAvailable() {
		t.Skip("docker is not available")
	}

	container, err := NewContainer(
		name,
		"alpine",
		WithTag("latest"),
		WithCommand("sh", "-c", "while true; do sleep 1; done"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := container.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = container.Cleanup(context.WithoutCancel(ctx))
	})

	return container
}
