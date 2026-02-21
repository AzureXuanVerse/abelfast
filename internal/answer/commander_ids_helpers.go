package answer

func normalizeCommanderIDs(ids []uint32, exclude uint32) []uint32 {
	if len(ids) == 0 {
		return nil
	}
	normalized := make([]uint32, 0, len(ids))
	seen := make(map[uint32]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 || id == exclude {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}
