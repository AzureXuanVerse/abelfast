package answer

import (
	answermeta "github.com/ggmolly/belfast/internal/answer/meta"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
)

func ensureCommanderMetaLoaded(commander *orm.Commander) error {
	return answermeta.EnsureCommanderMetaLoaded(commander)
}

func metaSkillSlots(ship *orm.OwnedShip) ([]orm.MetaTacticsSkillSlot, map[uint32]uint32, error) {
	return answermeta.MetaSkillSlots(ship)
}

func MetaCharActiveEnergy(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharActiveEnergy(buffer, client)
}

func MetaCharacterRepairLegacy(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharacterRepairLegacy(buffer, client)
}

func MetaCharActiveEnergyLegacy(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharActiveEnergyLegacy(buffer, client)
}

func MetaCharacterUnlockShipLegacy(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharacterUnlockShipLegacy(buffer, client)
}

func MetaCharacterRepair(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharacterRepair(buffer, client)
}

func MetaCharacterTacticsInfoRequestCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharacterTacticsInfoRequestCommandResponse(buffer, client)
}

func MetaCharacterTacticsLevelUpCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharacterTacticsLevelUpCommandResponse(buffer, client)
}

func MetaCharacterTacticsRequestCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharacterTacticsRequestCommandResponse(buffer, client)
}

func MetaCharacterTacticsSwitchCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharacterTacticsSwitchCommandResponse(buffer, client)
}

func MetaCharacterTacticsUnlockCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharacterTacticsUnlockCommandResponse(buffer, client)
}

func MetaCharacterUnlockShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answermeta.MetaCharacterUnlockShip(buffer, client)
}
