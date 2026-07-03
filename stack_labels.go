package scaffold

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types/filters"
)

/*
Labels returns the labels that identify the stack in Docker. Service
labels are added separately for each child service.
*/
func (s *Stack) Labels() map[string]string {
	return s.stackLabels()
}

func (s *Stack) applyLabels() {
	for _, service := range s.services {
		attachable, ok := service.(LabelAttachable)
		if ok {
			attachable.SetLabels(s.serviceLabels(service))
		}
	}
}

func (s *Stack) applyNamePrefix() {
	prefix := s.generatedNamePrefix()
	if prefix == "" {
		return
	}

	for _, service := range s.services {
		attachable, ok := service.(NamePrefixAttachable)
		if ok {
			attachable.SetNamePrefix(prefix)
		}
	}
}

func (s *Stack) stackLabels() map[string]string {
	labels := map[string]string{
		LabelManagedBy: "scaffold",
		LabelStack:     s.name,
	}
	if s.runID != "" {
		labels[LabelRunID] = s.runID
	}

	labels = mergeLabels(labels, s.inheritedLabels)
	labels = mergeLabels(labels, s.userLabels)
	labels[LabelManagedBy] = "scaffold"
	if _, ok := s.inheritedLabels[LabelStack]; !ok {
		labels[LabelStack] = s.name
	}
	if s.runID != "" {
		if _, ok := s.inheritedLabels[LabelRunID]; !ok {
			labels[LabelRunID] = s.runID
		}
	}

	return labels
}

func (s *Stack) generatedNamePrefix() string {
	if s.namePrefix == "" {
		return ""
	}

	return fmt.Sprintf("%s-%s", s.namePrefix, s.name)
}

func (s *Stack) serviceLabels(service Service) map[string]string {
	labels := s.stackLabels()
	labels[LabelService] = service.Name()

	return labels
}

func (s *Stack) labelFilters() filters.Args {
	filterArgs := filters.NewArgs()
	for key, value := range s.stackLabels() {
		filterArgs.Add("label", fmt.Sprintf("%s=%s", key, value))
	}

	return filterArgs
}

func (s *Stack) ensureRunID() {
	if s.runID != "" {
		return
	}

	s.runID = fmt.Sprintf("%s-%d", s.name, time.Now().UnixNano())
}
