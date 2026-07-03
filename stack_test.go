package scaffold

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	scaffoldcontainer "github.com/hlfshell/scaffold/container"
	"github.com/hlfshell/scaffold/logs"
)

func TestStackCreatesServiceGroupsInCallOrder(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	db := &fakeService{name: "db", events: &events, lock: lock}
	api := &fakeService{name: "api", events: &events, lock: lock}

	stack := NewStack(
		"test",
		WithServices(db),
		WithServices(api),
	)

	err := stack.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	err = stack.Cleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"create:db", "create:api", "cleanup:api", "cleanup:db"}
	if !reflect.DeepEqual(events, expected) {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestStackUsesContextLifecycle(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	service := &fakeContextService{name: "db", events: &events, lock: lock}

	stack := NewStack("test", WithServices(service))

	err := stack.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = stack.Cleanup(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"create:db", "cleanup:db"}
	if !reflect.DeepEqual(events, expected) {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestStackCreateReturnsCanceledContext(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	service := &fakeContextService{name: "db", events: &events, lock: lock}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stack := NewStack("test", WithServices(service))

	err := stack.Create(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
}

func TestStackCreatesServicesInSameGroupInParallel(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	started := make(chan string, 2)
	release := make(chan struct{})
	db := &fakeService{name: "db", events: &events, lock: lock, started: started, release: release}
	queue := &fakeService{name: "queue", events: &events, lock: lock, started: started, release: release}

	stack := NewStack("test", WithServices(db, queue))

	errs := make(chan error, 1)
	go func() {
		errs <- stack.Create(context.Background())
	}()

	first := <-started
	second := <-started
	if first == second {
		t.Fatalf("expected two services to start, got %s twice", first)
	}

	close(release)

	err := <-errs
	if err != nil {
		t.Fatal(err)
	}
}

func TestStackPartialFailureCleanup(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	db := &fakeService{name: "db", events: &events, lock: lock}
	api := &fakeService{name: "api", events: &events, lock: lock, fail: true}

	stack := NewStack(
		"test",
		WithServices(db),
		WithServices(api),
	)

	err := stack.Create(context.Background())
	if err == nil {
		t.Fatal("expected create error")
	}

	expected := []string{"create:db", "create:api", "cleanup:api", "cleanup:db"}
	if !reflect.DeepEqual(events, expected) {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestStackRejectsDuplicateServiceNames(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	first := &fakeService{name: "db", events: &events, lock: lock}
	second := &fakeService{name: "db", events: &events, lock: lock}

	stack := NewStack("app", WithServices(first, second))

	err := stack.Create(context.Background())
	if err == nil {
		t.Fatal("expected duplicate service name error")
	}
}

func TestStackAppliesDefaultAndInheritedLabels(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	db := &fakeService{name: "db", events: &events, lock: lock}

	stack := NewStack(
		"app",
		WithRunID("app-dev"),
		WithInheritedLabel("qwerty", "set"),
		WithInheritedLabel("env", "local"),
		WithServices(db),
	)

	err := stack.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]string{
		LabelManagedBy: "scaffold",
		LabelStack:     "app",
		LabelRunID:     "app-dev",
		LabelService:   "db",
		"qwerty":       "set",
		"env":          "local",
	}
	if !reflect.DeepEqual(db.labels, expected) {
		t.Fatalf("unexpected labels: %#v", db.labels)
	}
}

func TestNestedStackInheritsLabels(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	db := &fakeService{name: "db", events: &events, lock: lock}

	child := NewStack("child", WithServices(db))
	parent := NewStack(
		"parent",
		WithInheritedLabel("qwerty", "set"),
		WithServices(child),
	)

	err := parent.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if db.labels[LabelManagedBy] != "scaffold" {
		t.Fatalf("unexpected labels: %#v", db.labels)
	}
	if db.labels[LabelStack] != "parent" {
		t.Fatalf("unexpected labels: %#v", db.labels)
	}
	if db.labels[LabelService] != "db" {
		t.Fatalf("unexpected labels: %#v", db.labels)
	}
	if db.labels["qwerty"] != "set" {
		t.Fatalf("unexpected labels: %#v", db.labels)
	}
	if db.labels[LabelRunID] == "" {
		t.Fatalf("unexpected labels: %#v", db.labels)
	}
}

func TestStackDoesNotHaveRunIDUntilCreate(t *testing.T) {
	stack := NewStack("app")

	labels := stack.Labels()
	if _, ok := labels[LabelRunID]; ok {
		t.Fatalf("did not expect run id before create: %#v", labels)
	}
}

func TestStackGeneratesRunIDOnCreate(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	db := &fakeService{name: "db", events: &events, lock: lock}

	stack := NewStack("app", WithServices(db))

	err := stack.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if db.labels[LabelRunID] == "" {
		t.Fatalf("expected generated run id: %#v", db.labels)
	}
}

func TestReservedUserLabelsDoNotOverrideStackLabels(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	db := &fakeService{name: "db", events: &events, lock: lock}

	stack := NewStack(
		"app",
		WithInheritedLabel(LabelStack, "wrong"),
		WithInheritedLabel(LabelManagedBy, "wrong"),
		WithServices(db),
	)

	err := stack.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if db.labels[LabelStack] != "app" {
		t.Fatalf("unexpected labels: %#v", db.labels)
	}
	if db.labels[LabelManagedBy] != "scaffold" {
		t.Fatalf("unexpected labels: %#v", db.labels)
	}
}

func TestStackAppliesNamePrefix(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	db := &fakeService{name: "db", events: &events, lock: lock}

	stack := NewStack(
		"app",
		WithNamePrefix("dev"),
		WithServices(db),
	)

	err := stack.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if db.prefix != "dev-app" {
		t.Fatalf("unexpected prefix: %s", db.prefix)
	}
}

func TestNestedStackExtendsNamePrefix(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	db := &fakeService{name: "db", events: &events, lock: lock}

	child := NewStack("data", WithServices(db))
	parent := NewStack(
		"app",
		WithNamePrefix("dev"),
		WithServices(child),
	)

	err := parent.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if db.prefix != "dev-app-data" {
		t.Fatalf("unexpected prefix: %s", db.prefix)
	}
}

func TestContainerAppliesNamePrefix(t *testing.T) {
	container, err := scaffoldcontainer.NewContainer("postgres", "postgres")
	if err != nil {
		t.Fatal(err)
	}

	container.SetNamePrefix("dev-app")
	container.SetNamePrefix("dev-app")

	if container.Name() != "dev-app-postgres" {
		t.Fatalf("unexpected container name: %s", container.Name())
	}
}

func TestStackEnvEndpointsAndEnvFile(t *testing.T) {
	events := []string{}
	lock := &sync.Mutex{}
	db := &fakeService{
		name:      "db",
		events:    &events,
		lock:      lock,
		env:       map[string]string{"DATABASE_URL": "postgres://localhost/test"},
		endpoints: map[string]string{"db": "localhost:5432"},
	}

	stack := NewStack("app", WithServices(db))

	env := stack.Env()
	if env["DATABASE_URL"] != "postgres://localhost/test" {
		t.Fatalf("unexpected env: %#v", env)
	}

	endpoint, ok := stack.Endpoint("db")
	if !ok || endpoint != "localhost:5432" {
		t.Fatalf("unexpected endpoint: %s %v", endpoint, ok)
	}

	path := t.TempDir() + "/.env.scaffold"
	err := stack.WriteEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "DATABASE_URL=postgres://localhost/test\n" {
		t.Fatalf("unexpected env file: %s", string(contents))
	}
}

type fakeLogService struct {
	name    string
	streams logs.LogStreams
}

func (f *fakeLogService) Name() string {
	return f.name
}

func (f *fakeLogService) Create(ctx context.Context) error {
	return nil
}

func (f *fakeLogService) Cleanup(ctx context.Context) error {
	return nil
}

func (f *fakeLogService) Logs(context.Context) (logs.LogStreams, error) {
	return f.streams, nil
}

func TestStackLogsPrefixesChildStreams(t *testing.T) {
	api := &fakeLogService{
		name: "api",
		streams: logs.LogStreams{
			"api": io.NopCloser(strings.NewReader("api")),
		},
	}
	postgres := &fakeLogService{
		name: "postgres",
		streams: logs.LogStreams{
			"postgres": io.NopCloser(strings.NewReader("postgres")),
		},
	}
	data := NewStack("data", WithServices(postgres))
	app := NewStack("app", WithServices(api), WithServices(data))

	streams, err := app.Logs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer streams.Close()

	if _, ok := streams.GetStream("api"); !ok {
		t.Fatal("expected api stream")
	}
	if _, ok := streams.GetStream("data", "postgres"); !ok {
		t.Fatal("expected data.postgres stream")
	}
	if _, ok := streams.GetStream("app", "data", "postgres"); ok {
		t.Fatal("did not expect root stack name to be prefixed")
	}
}

func TestLogStreamsGetStreamJoinsNames(t *testing.T) {
	streams := logs.LogStreams{
		"data.postgres": io.NopCloser(strings.NewReader("postgres")),
	}
	defer streams.Close()

	stream, ok := streams.GetStream("data", "postgres")
	if !ok {
		t.Fatal("expected stream")
	}

	value, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "postgres" {
		t.Fatalf("expected postgres, got %s", string(value))
	}
}

type fakeService struct {
	name      string
	fail      bool
	created   bool
	events    *[]string
	lock      *sync.Mutex
	started   chan string
	release   chan struct{}
	labels    map[string]string
	env       map[string]string
	endpoints map[string]string
	prefix    string
}

func (f *fakeService) Name() string {
	return f.name
}

func (f *fakeService) Create(ctx context.Context) error {
	f.record("create:" + f.name)

	if f.started != nil {
		f.started <- f.name
	}
	if f.release != nil {
		<-f.release
	}

	if f.fail {
		return errors.New("create failed")
	}

	f.created = true
	return nil
}

func (f *fakeService) Cleanup(ctx context.Context) error {
	f.record("cleanup:" + f.name)
	return nil
}

func (f *fakeService) Logs(ctx context.Context) (logs.LogStreams, error) {
	return logs.LogStreams{}, ctx.Err()
}

func (f *fakeService) SetLabels(labels map[string]string) {
	f.labels = cloneLabels(labels)
}

func (f *fakeService) SetNamePrefix(prefix string) {
	f.prefix = prefix
}

func (f *fakeService) Env() map[string]string {
	return cloneLabels(f.env)
}

func (f *fakeService) Endpoints() map[string]string {
	return cloneLabels(f.endpoints)
}

func (f *fakeService) record(event string) {
	if f.lock != nil {
		f.lock.Lock()
		defer f.lock.Unlock()
	}

	*f.events = append(*f.events, event)
}

type fakeContextService struct {
	name    string
	events  *[]string
	lock    *sync.Mutex
	create  error
	cleanup error
}

func (f *fakeContextService) Name() string {
	return f.name
}

func (f *fakeContextService) Create(ctx context.Context) error {
	f.record("create:" + f.name)
	if f.create != nil {
		return f.create
	}

	return ctx.Err()
}

func (f *fakeContextService) Cleanup(ctx context.Context) error {
	f.record("cleanup:" + f.name)
	if f.cleanup != nil {
		return f.cleanup
	}

	return ctx.Err()
}

func (f *fakeContextService) Logs(ctx context.Context) (logs.LogStreams, error) {
	return logs.LogStreams{}, ctx.Err()
}

func (f *fakeContextService) record(event string) {
	if f.lock != nil {
		f.lock.Lock()
		defer f.lock.Unlock()
	}

	*f.events = append(*f.events, event)
}
