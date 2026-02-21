package medal

func containsUint32(list []uint32, value uint32) bool {
	for _, existing := range list {
		if existing == value {
			return true
		}
	}
	return false
}
