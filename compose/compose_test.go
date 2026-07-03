package compose

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCreateUsesWaitByDefault(t *testing.T) {
	var gotBinary string
	var gotArgs []string

	service, err := New("App Dev",
		WithFile("compose.yml"),
		withRunner(func(ctx context.Context, binary string, args []string, stdout io.Writer, stderr io.Writer) error {
			gotBinary = binary
			gotArgs = append([]string(nil), args...)
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = service.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if gotBinary != "docker" {
		t.Fatalf("unexpected binary: %s", gotBinary)
	}

	expected := []string{"compose", "-p", "app-dev", "-f", "compose.yml", "up", "-d", "--wait", "--wait-timeout", "120"}
	assertArgs(t, gotArgs, expected)
}

func TestCreateCanDisableWait(t *testing.T) {
	var gotArgs []string

	service, err := New("app",
		WithFile("compose.yml"),
		WithoutWait(),
		withRunner(func(ctx context.Context, binary string, args []string, stdout io.Writer, stderr io.Writer) error {
			gotArgs = append([]string(nil), args...)
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = service.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"compose", "-p", "app", "-f", "compose.yml", "up", "-d"}
	assertArgs(t, gotArgs, expected)
}

func TestCreateUsesConfiguredWaitTimeout(t *testing.T) {
	var gotArgs []string

	service, err := New("app",
		WithFile("compose.yml"),
		WithWaitTimeout(45*time.Second),
		withRunner(func(ctx context.Context, binary string, args []string, stdout io.Writer, stderr io.Writer) error {
			gotArgs = append([]string(nil), args...)
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = service.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"compose", "-p", "app", "-f", "compose.yml", "up", "-d", "--wait", "--wait-timeout", "45"}
	assertArgs(t, gotArgs, expected)
}

func TestEmbeddedFileIsWrittenForCommand(t *testing.T) {
	const content = "services:\n  web:\n    image: nginx:alpine\n"

	var composePath string
	service, err := New("app",
		WithEmbeddedFile("compose.yml", []byte(content)),
		withRunner(func(ctx context.Context, binary string, args []string, stdout io.Writer, stderr io.Writer) error {
			for i, arg := range args {
				if arg == "-f" && i+1 < len(args) {
					composePath = args[i+1]
				}
			}

			contents, err := os.ReadFile(composePath)
			if err != nil {
				return err
			}
			if string(contents) != content {
				t.Fatalf("unexpected embedded compose content: %s", string(contents))
			}

			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = service.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if composePath == "" {
		t.Fatal("expected embedded compose file path")
	}
	if _, err := os.Stat(composePath); !os.IsNotExist(err) {
		t.Fatalf("expected embedded compose temp file to be removed, got %v", err)
	}
}

func TestReadyCheckRunsAfterCompose(t *testing.T) {
	ready := false
	service, err := New("app",
		WithFile("compose.yml"),
		WithReadyCheck(func(ctx context.Context, compose *Compose) error {
			ready = true
			return nil
		}),
		withRunner(func(ctx context.Context, binary string, args []string, stdout io.Writer, stderr io.Writer) error {
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	err = service.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Fatal("expected ready check to run")
	}
}

func assertArgs(t *testing.T, got []string, expected []string) {
	t.Helper()

	if strings.Join(got, "\x00") != strings.Join(expected, "\x00") {
		t.Fatalf("unexpected args:\n got: %#v\nwant: %#v", got, expected)
	}
}
