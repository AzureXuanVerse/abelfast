package meta

import "github.com/ggmolly/belfast/internal/orm"

func EnsureCommanderMetaLoaded(commander *orm.Commander) error {
	return ensureCommanderMetaLoaded(commander)
}

func MetaSkillSlots(ship *orm.OwnedShip) ([]orm.MetaTacticsSkillSlot, map[uint32]uint32, error) {
	return metaSkillSlots(ship)
}
