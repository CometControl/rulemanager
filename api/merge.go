package api

// deepMergeJSON recursively merges updates into existing map.
// For nested maps, it merges recursively.
// For other types (including arrays), it replaces the value.
func deepMergeJSON(existing, updates map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy all existing values
	for k, v := range existing {
		result[k] = v
	}

	// Merge updates
	for k, updateValue := range updates {
		existingValue, exists := result[k]

		// If both are maps, merge recursively
		if exists {
			existingMap, existingIsMap := existingValue.(map[string]interface{})
			updateMap, updateIsMap := updateValue.(map[string]interface{})

			if existingIsMap && updateIsMap {
				result[k] = deepMergeJSON(existingMap, updateMap)
				continue
			}
		}

		// Otherwise, replace the value
		result[k] = updateValue
	}

	return result
}
