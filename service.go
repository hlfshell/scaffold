package scaffold

import (
	"context"
	"time"

	"github.com/hlfshell/scaffold/logs"
)

/*
Service is the minimum lifecycle interface used by Stack. Services only need a
name, a create step, and a cleanup step to be composable.
*/
type Service interface {
	Name() string
	Create(ctx context.Context) error
	Cleanup(ctx context.Context) error
	Logs(ctx context.Context) (logs.LogStreams, error)
}

/*
Connectable is implemented by harnesses that can return a typed client
or connection after the service is running.
*/
type Connectable[T any] interface {
	Connect() (T, error)
	ConnectWithTimeout(timeout time.Duration) (T, error)
}

/*
NetworkAttachable is implemented by harnesses that can join a shared
Docker network before they are created.
*/
type NetworkAttachable interface {
	SetNetwork(name string)
}

/*
LabelAttachable is implemented by harnesses that can receive Docker
labels from a parent stack before they are created.
*/
type LabelAttachable interface {
	SetLabels(labels map[string]string)
}

/*
NamePrefixAttachable is implemented by harnesses that can apply a stack
name prefix to Docker resources before they are created.
*/
type NamePrefixAttachable interface {
	SetNamePrefix(prefix string)
}

/*
EnvProvider is implemented by services that can export environment
variables for applications, tests, or CLI commands.
*/
type EnvProvider interface {
	Env() map[string]string
}

/*
EndpointProvider is implemented by services that can expose named local
endpoints after creation.
*/
type EndpointProvider interface {
	Endpoints() map[string]string
}
