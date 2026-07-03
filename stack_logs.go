package scaffold

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hlfshell/scaffold/logs"
)

/*
Logs returns named log streams for every service in the stack. Child stack
and service names are used as path segments, so callers can choose streams
such as "api" or "data.postgres".
*/
func (s *Stack) Logs(ctx context.Context) (logs.LogStreams, error) {
	return collectServiceLogs(ctx, s.services)
}

/*
CollectLogs returns the named log streams exposed by a service. For stacks,
this recursively returns streams for child services and containers.
*/
func CollectLogs(ctx context.Context, service Service) (logs.LogStreams, error) {
	if service == nil {
		return nil, fmt.Errorf("service is nil")
	}

	return service.Logs(ctx)
}

/*
MergedLogs returns a single reader containing all logs exposed by a service.
Use CollectLogs when callers need to choose individual named streams.
*/
func MergedLogs(ctx context.Context, service Service) (io.ReadCloser, error) {
	streams, err := CollectLogs(ctx, service)
	if err != nil {
		return nil, err
	}

	return logs.Merge(streams), nil
}

func collectServiceLogs(ctx context.Context, services []Service) (logs.LogStreams, error) {
	streams := logs.LogStreams{}

	for _, service := range services {
		if service == nil {
			streams.Close()
			return nil, fmt.Errorf("service is nil")
		}

		serviceStreams, err := service.Logs(ctx)
		if err != nil {
			streams.Close()
			return nil, fmt.Errorf("failed to open logs for service %s: %w", service.Name(), err)
		}

		for name, stream := range serviceStreams {
			streams[logs.UniqueName(streams, childLogName(service.Name(), name))] = stream
		}
	}

	return streams, nil
}

func childLogName(serviceName string, streamName string) string {
	if streamName == "" || streamName == serviceName {
		return serviceName
	}
	if strings.HasPrefix(streamName, serviceName+".") {
		return streamName
	}

	return serviceName + "." + streamName
}
