package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func IslandChangeCommanderDress(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21626
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21627, err
	}

	response := &protobuf.SC_21627{Result: proto.Uint32(1), CapList: []*protobuf.PB_CAP_STATE{}}
	if err := ensureCommanderLoaded(client, "Island/ChangeCommanderDress"); err != nil {
		return client.SendMessage(21627, response)
	}

	islandID := payload.GetIslandId()
	if islandID == 0 {
		islandID = client.Commander.CommanderID
	}
	if islandID != client.Commander.CommanderID {
		return client.SendMessage(21627, response)
	}

	for i := range payload.GetColorList() {
		dressID := payload.GetColorList()[i].GetId()
		if dressID == 0 {
			continue
		}
		state, err := orm.GetIslandRoleDressState(client.Commander.CommanderID, dressID)
		if err != nil || state.Num == 0 {
			continue
		}
		_ = orm.UpsertIslandCommanderDressState(&orm.IslandCommanderDressState{
			CommanderID: client.Commander.CommanderID,
			DressID:     dressID,
			State:       1,
			Color:       payload.GetColorList()[i].GetColor(),
			ColorList:   []uint32{payload.GetColorList()[i].GetColor()},
		})
	}

	curDress := make([]orm.IslandCurDress, 0, len(payload.GetDressList()))
	for i := range payload.GetDressList() {
		if payload.GetDressList()[i].GetType() == 0 {
			continue
		}
		curDress = append(curDress, orm.IslandCurDress{Type: payload.GetDressList()[i].GetType(), ID: payload.GetDressList()[i].GetId()})
	}
	profile := &orm.IslandCommanderDressProfile{
		CommanderID: client.Commander.CommanderID,
		IslandID:    islandID,
		CurDress:    curDress,
		CapList:     []orm.IslandCapState{},
	}
	if err := orm.UpsertIslandCommanderDressProfile(profile); err != nil {
		return client.SendMessage(21627, response)
	}

	response.Result = proto.Uint32(0)
	return client.SendMessage(21627, response)
}
