package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hlfshell/scaffold"
	"github.com/hlfshell/scaffold/logs"
)

type fakeService struct {
	name      string
	created   bool
	cleaned   bool
	downed    bool
	env       map[string]string
	endpoints map[string]string
}

func (f *fakeService) Name() string {
	return f.name
}

func (f *fakeService) Create(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	f.created = true
	return nil
}

func (f *fakeService) Cleanup(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	f.cleaned = true
	return nil
}

func (f *fakeService) Logs(ctx context.Context) (logs.LogStreams, error) {
	return logs.LogStreams{}, ctx.Err()
}

func (f *fakeService) Env() map[string]string {
	return f.env
}

func (f *fakeService) Endpoints() map[string]string {
	return f.endpoints
}

func (f *fakeService) Down(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	f.downed = true
	return nil
}

func TestOnceCreatesPrintsAndCleans(t *testing.T) {
	service := &fakeService{
		name:      "app",
		env:       map[string]string{"APP_URL": "http://localhost:8080"},
		endpoints: map[string]string{"app": "http://localhost:8080"},
	}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := New("dev", service, WithWriters(out, errOut))

	code := app.Run([]string{"once", "--no-env"})
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, errOut.String())
	}
	if !service.created || !service.cleaned {
		t.Fatalf("expected create and cleanup, got created=%v cleaned=%v", service.created, service.cleaned)
	}
	if !strings.Contains(out.String(), "APP_URL=http://localhost:8080") {
		t.Fatalf("expected env output, got %s", out.String())
	}
}

func TestUpCreatesWritesEnvAndLeavesRunning(t *testing.T) {
	service := &fakeService{
		name: "app",
		env:  map[string]string{"APP_URL": "http://localhost:8080"},
	}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	path := filepath.Join(t.TempDir(), ".env.example")
	app := New("dev", service, WithWriters(out, errOut), WithEnvFile(path))

	code := app.Run([]string{"up"})
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, errOut.String())
	}
	if !service.created {
		t.Fatal("expected service to be created")
	}
	if service.cleaned {
		t.Fatal("did not expect up to clean up")
	}
	if out.String() != "dev up\n" {
		t.Fatalf("unexpected output: %q", out.String())
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "APP_URL=http://localhost:8080\n" {
		t.Fatalf("unexpected env file: %s", string(contents))
	}
}

func TestRunWithoutCommandsListsCommands(t *testing.T) {
	service := &fakeService{name: "app"}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := New("dev", service, WithWriters(out, errOut))

	code := app.Run([]string{"run"})
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if !strings.Contains(out.String(), "Commands:") || !strings.Contains(out.String(), "none") {
		t.Fatalf("unexpected stdout: %s", out.String())
	}
}

func TestRunExecutesRegisteredCommand(t *testing.T) {
	service := &fakeService{name: "app"}
	called := false
	app := New("dev", service,
		WithWriters(&bytes.Buffer{}, &bytes.Buffer{}),
		WithCommand(Command{
			Name: "seed",
			Help: "seed test data",
			Run: func(ctx context.Context, service scaffold.Service, args []string) error {
				called = true
				if len(args) != 1 || args[0] != "users" {
					t.Fatalf("unexpected args: %#v", args)
				}
				return nil
			},
		}),
	)

	code := app.Run([]string{"run", "--no-env", "seed", "users"})
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if !called {
		t.Fatal("expected command to run")
	}
	if !service.created || !service.cleaned {
		t.Fatalf("expected lifecycle around command, got created=%v cleaned=%v", service.created, service.cleaned)
	}
}

func TestDownUsesDown(t *testing.T) {
	service := &fakeService{name: "app"}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := New("dev", service, WithWriters(out, errOut))

	code := app.Run([]string{"down"})
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, errOut.String())
	}
	if !service.downed {
		t.Fatal("expected down to be called")
	}
	if !strings.Contains(out.String(), "Stopped and removed matching resources.") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestHelpReturnsSuccess(t *testing.T) {
	service := &fakeService{name: "app"}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := New("dev", service, WithWriters(out, errOut))

	code := app.Run([]string{"--help"})
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Usage: dev <command>") {
		t.Fatalf("expected help output, got %s", out.String())
	}
	if strings.Contains(out.String(), "run [<command>") {
		t.Fatalf("did not expect run without commands: %s", out.String())
	}
	if strings.Contains(out.String(), "endpoints") {
		t.Fatalf("did not expect endpoints without endpoints: %s", out.String())
	}
}

func TestHelpShowsCommandsAndEndpointsWhenAvailable(t *testing.T) {
	service := &fakeService{name: "app", endpoints: map[string]string{"app": "http://localhost:8080"}}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := New("dev", service,
		WithWriters(out, errOut),
		WithCommand(Command{Name: "seed", Help: "seed data", Run: func(context.Context, scaffold.Service, []string) error {
			return nil
		}}),
	)

	code := app.Run([]string{"--help"})
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "run [<command>") {
		t.Fatalf("expected run with commands: %s", out.String())
	}
	if !strings.Contains(out.String(), "endpoints") {
		t.Fatalf("expected endpoints with endpoints: %s", out.String())
	}
}

func TestNoArgsShowsHelp(t *testing.T) {
	service := &fakeService{name: "app"}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := New("dev", service, WithWriters(out, errOut))

	code := app.Run(nil)
	if code != 0 {
		t.Fatalf("unexpected exit code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Usage: dev <command>") {
		t.Fatalf("expected help output, got %s", out.String())
	}
}

func TestParseErrorShowsUsage(t *testing.T) {
	service := &fakeService{name: "app"}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := New("dev", service, WithWriters(out, errOut))

	code := app.Run([]string{"missing"})
	if code != 2 {
		t.Fatalf("unexpected exit code %d", code)
	}
	if !strings.Contains(out.String(), "Usage: dev <command>") {
		t.Fatalf("expected usage output, got %s", out.String())
	}
	if !strings.Contains(errOut.String(), "unexpected argument") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestOnceUsesAppEnvFileDefault(t *testing.T) {
	service := &fakeService{
		name: "app",
		env:  map[string]string{"APP_URL": "http://localhost:8080"},
	}
	path := filepath.Join(t.TempDir(), ".env.example")
	app := New("dev", service, WithWriters(&bytes.Buffer{}, &bytes.Buffer{}), WithEnvFile(path))

	code := app.Run([]string{"once"})
	if code != 0 {
		t.Fatalf("unexpected exit code %d", code)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "APP_URL=http://localhost:8080\n" {
		t.Fatalf("unexpected env file: %s", string(contents))
	}
}

func TestCreateUsesContextCancellation(t *testing.T) {
	service := &fakeService{name: "app"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := create(ctx, service)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestLiveCommandsPageShowsRegisteredCommands(t *testing.T) {
	service := &fakeService{name: "app"}
	app := New("dev",
		service,
		WithWriters(&bytes.Buffer{}, &bytes.Buffer{}),
		WithCommand(Command{
			Name: "seed",
			Help: "seed data",
			Run: func(context.Context, scaffold.Service, []string) error {
				return nil
			},
		}),
	)
	app.commands = normalizeCommands(app.service, app.commands)

	model := newModel(app)
	model.tab = tabCommands

	page := model.page()
	if !strings.Contains(page, "seed") || !strings.Contains(page, "seed data") {
		t.Fatalf("expected command page to include registered command, got %s", page)
	}
}
