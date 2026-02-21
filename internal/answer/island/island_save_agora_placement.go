package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandSaveAgoraPlacement(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21307
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21308, err
	}

	response := &protobuf.SC_21308{Result: proto.Uint32(1)}
	if payload.GetUpdateData() == nil {
		return client.SendMessage(21308, response)
	}

	normalized := &protobuf.PB_PLACEMENT_DATA{
		PlacedList: make([]*protobuf.PB_FURNITURE_DATA, 0, len(payload.GetUpdateData().GetPlacedList())),
		FloorData:  append([]uint32(nil), payload.GetUpdateData().GetFloorData()...),
		TileData:   append([]uint32(nil), payload.GetUpdateData().GetTileData()...),
	}
	for _, row := range payload.GetUpdateData().GetPlacedList() {
		if row == nil {
			continue
		}
		normalized.PlacedList = append(normalized.PlacedList, &protobuf.PB_FURNITURE_DATA{
			Id:  proto.Uint32(row.GetId()),
			X:   proto.Int32(row.GetX()),
			Y:   proto.Int32(row.GetY()),
			Dir: proto.Uint32(row.GetDir()),
		})
	}

	placedData, err := proto.Marshal(normalized)
	if err != nil {
		return client.SendMessage(21308, response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.UpsertIslandAgoraPlacementTx(context.Background(), tx, client.Commander.CommanderID, placedData)
	})
	if err != nil {
		return client.SendMessage(21308, response)
	}

	response.Result = proto.Uint32(0)
	if _, _, err := client.SendMessage(21308, response); err != nil {
		return 0, 21308, err
	}

	push := &protobuf.SC_21309{IslandId: proto.Uint32(client.Commander.CommanderID), UpdateData: normalized}
	broadcastIslandPacketExcept(client.Server, client.Commander.CommanderID, client.Commander.CommanderID, 21309, push)
	return 0, 21308, nil
}

func broadcastIslandPacketExcept(server *connection.Server, islandID uint32, excludeCommanderID uint32, packetID int, message proto.Message) {
	if server == nil {
		return
	}
	for _, candidate := range server.ListClients() {
		if candidate == nil || candidate.Commander == nil {
			continue
		}
		if candidate.Commander.CommanderID == excludeCommanderID {
			continue
		}
		if globalIslandRuntimeState.hasMatchingSession(candidate.Commander.CommanderID, islandID) {
			candidate.SendMessage(packetID, message)
		}
	}
}
