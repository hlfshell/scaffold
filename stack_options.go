package scaffold

import "fmt"

/*
WithRunID sets the run identity used when finding existing Docker
resources for this stack. If not set, a run id is generated when Create
starts.
*/
func WithRunID(runID string) StackOption {
	return func(s *Stack) {
		s.runID = runID
	}
}

/*
WithNamePrefix prefixes Docker resource names for services that support
SetNamePrefix. For stack "app" and prefix "dev", child service resources
receive the prefix "dev-app".
*/
func WithNamePrefix(prefix string) StackOption {
	return func(s *Stack) {
		s.namePrefix = prefix
	}
}

/*
WithInheritedLabel adds a key/value inherited label. Inherited labels are
pushed down to child services and child stacks.
*/
func WithInheritedLabel(label string, value string) StackOption {
	return func(s *Stack) {
		if isReservedLabel(label) {
			return
		}

		s.userLabels[label] = value
	}
}

/*
WithInheritedLabels adds multiple inherited labels to the stack.
*/
func WithInheritedLabels(labels map[string]string) StackOption {
	return func(s *Stack) {
		s.userLabels = mergeUserLabels(s.userLabels, labels)
	}
}

/*
WithServices adds a group of services to the stack. Services passed in
the same call are created in parallel. Separate WithServices calls are
created in call order.
*/
func WithServices(services ...Service) StackOption {
	return func(s *Stack) {
		if len(services) == 0 {
			return
		}

		group := make([]Service, len(services))
		copy(group, services)

		s.serviceGroups = append(s.serviceGroups, group)
		s.services = append(s.services, services...)
	}
}

/*
WithSharedNetwork creates a Docker network for the stack and attaches
services that support SetNetwork before they are created.
*/
func WithSharedNetwork() StackOption {
	return func(s *Stack) {
		s.sharedNetwork = true
		s.networkName = fmt.Sprintf("scaffold-%s", s.name)
	}
}
