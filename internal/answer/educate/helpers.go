package educate

func boolToUint32(value bool) uint32 {
	if value {
		return 1
	}
	return 0
}

func appendUniqueUint32(values []uint32, value uint32) []uint32 {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsUint32(values []uint32, target uint32) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
