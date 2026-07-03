package scaffold

const (
	LabelManagedBy = "scaffold.managed-by"
	LabelStack     = "scaffold.stack"
	LabelService   = "scaffold.service"
	LabelRunID     = "scaffold.run-id"
)

var reservedLabels = map[string]struct{}{
	LabelManagedBy: {},
	LabelStack:     {},
	LabelService:   {},
	LabelRunID:     {},
}

func cloneLabels(labels map[string]string) map[string]string {
	cloned := map[string]string{}
	for key, value := range labels {
		cloned[key] = value
	}

	return cloned
}

func mergeLabels(base map[string]string, labels map[string]string) map[string]string {
	merged := cloneLabels(base)
	for key, value := range labels {
		merged[key] = value
	}

	return merged
}

func mergeUserLabels(base map[string]string, labels map[string]string) map[string]string {
	merged := cloneLabels(base)
	for key, value := range labels {
		if isReservedLabel(key) {
			continue
		}

		merged[key] = value
	}

	return merged
}

func isReservedLabel(label string) bool {
	_, ok := reservedLabels[label]
	return ok
}
