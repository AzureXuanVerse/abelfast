package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func SendSpecialWeaponSync(client *connection.Client) (int, int, error) {
	response := protobuf.SC_14200{
		SpweaponList: orm.ToProtoOwnedSpWeaponList(client.Commander.OwnedSpWeapons),
	}
	if response.SpweaponList == nil {
		response.SpweaponList = []*protobuf.SPWEAPONINFO{}
	}

	return client.SendMessage(14200, &response)
}
