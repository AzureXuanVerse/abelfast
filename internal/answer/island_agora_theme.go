package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func SaveIslandAgoraTheme(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21317
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21318, err
	}

	theme := request.GetTheme()
	if theme == nil || theme.GetId() == 0 || theme.GetPlacedData() == nil {
		response := protobuf.SC_21318{Result: proto.Uint32(1)}
		return client.SendMessage(21318, &response)
	}

	placedDataBytes, err := proto.Marshal(theme.GetPlacedData())
	if err != nil {
		response := protobuf.SC_21318{Result: proto.Uint32(1)}
		return client.SendMessage(21318, &response)
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		return orm.UpsertIslandAgoraThemeTx(ctx, tx, client.Commander.CommanderID, theme.GetId(), theme.GetName(), placedDataBytes)
	})
	if err != nil {
		response := protobuf.SC_21318{Result: proto.Uint32(1)}
		return client.SendMessage(21318, &response)
	}

	response := protobuf.SC_21318{Result: proto.Uint32(0)}
	return client.SendMessage(21318, &response)
}

func ListIslandAgoraThemes(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21321
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21322, err
	}

	themes, err := orm.ListIslandAgoraThemes(client.Commander.CommanderID)
	if err != nil {
		return 0, 21322, err
	}

	respThemes := make([]*protobuf.PB_PLACEMENT_THEME, 0, len(themes))
	for _, row := range themes {
		placedData := &protobuf.PB_PLACEMENT_DATA{PlacedList: []*protobuf.PB_FURNITURE_DATA{}, FloorData: []uint32{}, TileData: []uint32{}}
		if len(row.PlacedData) > 0 {
			decoded := &protobuf.PB_PLACEMENT_DATA{}
			if err := proto.Unmarshal(row.PlacedData, decoded); err == nil {
				placedData = decoded
			}
		}

		respThemes = append(respThemes, &protobuf.PB_PLACEMENT_THEME{
			Id:         proto.Uint32(row.ThemeSlotID),
			Name:       proto.String(row.Name),
			PlacedData: placedData,
		})
	}

	response := protobuf.SC_21322{ThemeList: respThemes}
	return client.SendMessage(21322, &response)
}

func DeleteIslandAgoraTheme(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21319
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21320, err
	}

	ctx := context.Background()
	err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		return orm.DeleteIslandAgoraThemeTx(ctx, tx, client.Commander.CommanderID, request.GetId())
	})
	if err != nil {
		response := protobuf.SC_21320{Result: proto.Uint32(1)}
		return client.SendMessage(21320, &response)
	}

	response := protobuf.SC_21320{Result: proto.Uint32(0)}
	return client.SendMessage(21320, &response)
}
