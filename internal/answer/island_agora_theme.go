package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

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
