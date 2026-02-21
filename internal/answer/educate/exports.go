package educate

func EducateFlagID(base uint32, id uint32) uint32 {
	return educateFlagID(base, id)
}

func HasEducateFlag(commanderID uint32, flagID uint32) (bool, error) {
	return hasEducateFlag(commanderID, flagID)
}
