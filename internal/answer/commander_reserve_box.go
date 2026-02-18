package answer

import (
	"encoding/json"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func CommanderReserveBox(buffer *[]byte, client *connection.Client) (int, int, error) {
	var packet protobuf.CS_25018
	if err := proto.Unmarshal(*buffer, &packet); err != nil {
		return 0, 25019, err
	}

	response := protobuf.SC_25019{
		Result: proto.Uint32(commanderResultError),
		Awards: []*protobuf.DROPINFO{},
	}
	count := packet.GetType()
	if count == 0 {
		return client.SendMessage(25019, &response)
	}
	costCurve := loadCommanderReserveCostCurve()
	usage := client.Commander.DrawCount1
	if usage+count > uint32(len(costCurve)) {
		return client.SendMessage(25019, &response)
	}
	totalCost := uint32(0)
	for i := usage; i < usage+count; i++ {
		totalCost += costCurve[i]
	}
	if !client.Commander.HasEnoughGold(totalCost) {
		return client.SendMessage(25019, &response)
	}
	if err := client.Commander.ConsumeResource(1, totalCost); err != nil {
		return 0, 25019, err
	}
	if err := client.Commander.IncrementReserveUsage(count); err != nil {
		return 0, 25019, err
	}
	if err := client.Commander.AddItem(20001, count); err != nil {
		return 0, 25019, err
	}

	awards := make([]*protobuf.DROPINFO, 0, count)
	for i := uint32(0); i < count; i++ {
		awards = append(awards, &protobuf.DROPINFO{
			Type:   proto.Uint32(2),
			Id:     proto.Uint32(20001),
			Number: proto.Uint32(1),
		})
	}

	response.Result = proto.Uint32(commanderResultOK)
	response.Awards = awards
	return client.SendMessage(25019, &response)
}

func loadCommanderReserveCostCurve() []uint32 {
	entry, err := orm.GetConfigEntry("ShareCfg/gameset.json", "commander_get_cost")
	if err != nil {
		return []uint32{300, 600, 900, 1200, 1500, 1800}
	}
	var payload struct {
		Description []uint32 `json:"description"`
	}
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return []uint32{300, 600, 900, 1200, 1500, 1800}
	}
	if len(payload.Description) == 0 {
		return []uint32{300, 600, 900, 1200, 1500, 1800}
	}
	return payload.Description
}
