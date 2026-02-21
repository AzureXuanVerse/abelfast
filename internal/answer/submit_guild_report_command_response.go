package answer

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func SubmitGuildReportCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61019
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61020, err
	}
	response := &protobuf.SC_61020{Result: proto.Uint32(guildEventResultFailure), DropList: []*protobuf.DROPINFO{}}
	if client.Commander == nil {
		return client.SendMessage(61020, response)
	}
	ids := stableUniqueNonZero(payload.GetIds())
	if len(ids) == 0 {
		return client.SendMessage(61020, response)
	}
	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(61020, response)
	}

	dropMap := map[string]*protobuf.DROPINFO{}
	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		reports, err := orm.ClaimGuildReportsTx(context.Background(), tx, guild.ID, ids)
		if err != nil {
			return err
		}
		for _, report := range reports {
			if report.DropCount == 0 {
				continue
			}
			accumulateDrop(dropMap, report.DropType, report.DropID, report.DropCount)
		}
		if len(dropMap) == 0 {
			return nil
		}
		return applyLoveLetterDropsTx(context.Background(), tx, client, dropMap)
	})
	if err != nil {
		return client.SendMessage(61020, response)
	}

	response.Result = proto.Uint32(guildEventResultSuccess)
	response.DropList = dropMapToSortedList(dropMap)
	return client.SendMessage(61020, response)
}

func stableUniqueNonZero(ids []uint32) []uint32 {
	seen := make(map[uint32]struct{}, len(ids))
	out := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
