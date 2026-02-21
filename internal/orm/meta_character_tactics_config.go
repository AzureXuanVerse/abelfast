package orm

type MetaTacticsSkillSlot struct {
	SkillID uint32
	Pos     uint32
}

func GetMetaTacticsSkillSlotsByShipTemplate(shipTemplateID uint32) ([]MetaTacticsSkillSlot, error) {
	shipCfg, err := GetShipDataTemplateMetaConfig(shipTemplateID)
	if err != nil {
		return nil, err
	}
	result := make([]MetaTacticsSkillSlot, 0)
	for _, buffID := range shipCfg.BuffListDisplay {
		if _, err := GetShipMetaSkillTaskConfig(buffID, 1); err != nil {
			continue
		}
		result = append(result, MetaTacticsSkillSlot{SkillID: buffID, Pos: uint32(len(result) + 1)})
	}
	return result, nil
}
