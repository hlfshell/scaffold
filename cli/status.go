package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hlfshell/scaffold"
)

type StatusReport struct {
	Name      string
	Known     bool
	Running   bool
	Services  []ServiceStatus
	Resources scaffold.ResourceStatus
}

type ServiceStatus struct {
	Path    string
	Running bool
	Known   bool
}

func status(ctx context.Context, service scaffold.Service) (StatusReport, error) {
	report := StatusReport{
		Name: service.Name(),
	}

	resources, ok, err := resources(ctx, service)
	if err != nil {
		return report, err
	}
	if ok {
		report.Known = true
		report.Resources = resources
		report.Running = anyContainerRunning(resources)
		report.Services = serviceStatuses(service, resources, "", true)
		return report, nil
	}

	checker, ok := service.(interface {
		IsRunning(context.Context) (bool, error)
	})
	if ok {
		running, err := checker.IsRunning(ctx)
		if err != nil {
			return report, err
		}
		report.Known = true
		report.Running = running
		report.Services = []ServiceStatus{{Path: service.Name(), Running: running, Known: true}}
		return report, nil
	}

	report.Services = serviceStatuses(service, scaffold.ResourceStatus{}, "", false)
	return report, nil
}

func printStatus(out io.Writer, report StatusReport) {
	state := "unknown"
	if report.Known {
		if report.Running {
			state = "running"
		} else {
			state = "stopped"
		}
	}

	fmt.Fprintf(out, "Status: %s\n", state)

	if len(report.Services) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Services:")
		for _, service := range report.Services {
			state := "unknown"
			if service.Known {
				if service.Running {
					state = "running"
				} else {
					state = "stopped"
				}
			}
			fmt.Fprintf(out, "  %-28s %s\n", service.Path, state)
		}
	}

	if report.Known {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Resources:")
		fmt.Fprintf(out, "  containers: %d running / %d total\n", runningContainerCount(report.Resources), len(report.Resources.Containers))
		fmt.Fprintf(out, "  networks:   %d\n", len(report.Resources.Networks))
		fmt.Fprintf(out, "  volumes:    %d\n", len(report.Resources.Volumes))
	}
}

func serviceStatuses(service scaffold.Service, resources scaffold.ResourceStatus, prefix string, known bool) []ServiceStatus {
	path := service.Name()
	if prefix != "" {
		path = prefix + "/" + service.Name()
	}

	childrenProvider, ok := service.(interface {
		Services() []scaffold.Service
	})
	if ok {
		statuses := []ServiceStatus{}
		for _, child := range childrenProvider.Services() {
			statuses = append(statuses, serviceStatuses(child, resources, path, known)...)
		}
		return statuses
	}

	return []ServiceStatus{{
		Path:    path,
		Running: serviceRunning(service, resources),
		Known:   known,
	}}
}

func serviceRunning(service scaffold.Service, resources scaffold.ResourceStatus) bool {
	for _, container := range resources.Containers {
		if container.Labels[scaffold.LabelService] == service.Name() && container.Running {
			return true
		}
	}

	return false
}

func anyContainerRunning(resources scaffold.ResourceStatus) bool {
	return runningContainerCount(resources) > 0
}

func runningContainerCount(resources scaffold.ResourceStatus) int {
	count := 0
	for _, container := range resources.Containers {
		if container.Running {
			count++
		}
	}

	return count
}

func statusText(report StatusReport) string {
	var builder strings.Builder
	printStatus(&builder, report)
	return strings.TrimRight(builder.String(), "\n")
}
