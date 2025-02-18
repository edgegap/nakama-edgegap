package helpers

func AppendIfNotExists(slice []string, item string) []string {
	for _, existingItem := range slice {
		if existingItem == item {
			return slice // Item already exists, return the original slice
		}
	}
	return append(slice, item) // Item not found, append it
}

func MergeMaps[K comparable, V any](m1, m2 map[K]V) map[K]V {
	merged := make(map[K]V)

	for k, v := range m1 {
		merged[k] = v
	}
	for k, v := range m2 {
		merged[k] = v
	}

	return merged
}

func RemoveElements(source, remove []string) []string {
	removeMap := make(map[string]struct{}, len(remove))
	for _, r := range remove {
		removeMap[r] = struct{}{}
	}

	result := make([]string, 0, len(source))
	for _, s := range source {
		if _, found := removeMap[s]; !found {
			result = append(result, s)
		}
	}
	return result
}
