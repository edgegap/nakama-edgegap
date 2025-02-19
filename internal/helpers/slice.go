package helpers

// AppendIfNotExists adds an item to the slice if it does not already exist.
// It returns the modified slice.
func AppendIfNotExists(slice []string, item string) []string {
	for _, existingItem := range slice {
		if existingItem == item {
			return slice // Item already exists, return the original slice
		}
	}
	return append(slice, item) // Item not found, append it
}

// MergeMaps merges two maps into a new map.
// If both maps contain the same key, the value from m2 takes precedence.
func MergeMaps[K comparable, V any](m1, m2 map[K]V) map[K]V {
	merged := make(map[K]V)

	// Copy all elements from the first map
	for k, v := range m1 {
		merged[k] = v
	}
	// Copy all elements from the second map, overwriting duplicates
	for k, v := range m2 {
		merged[k] = v
	}

	return merged
}

// RemoveElements removes all occurrences of elements in the 'remove' slice from the 'source' slice.
// It returns a new slice containing only the elements that are not in 'remove'.
func RemoveElements(source, remove []string) []string {
	// Create a set of elements to remove for quick lookup
	removeMap := make(map[string]struct{}, len(remove))
	for _, r := range remove {
		removeMap[r] = struct{}{}
	}

	// Build the result slice with elements not in removeMap
	result := make([]string, 0, len(source))
	for _, s := range source {
		if _, found := removeMap[s]; !found {
			result = append(result, s)
		}
	}
	return result
}
