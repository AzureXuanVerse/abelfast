package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandAccessOpSetWhiteList = uint32(1)
	islandAccessOpSetBlackList = uint32(2)
	islandAccessOpKick         = uint32(3)
	islandAccessOpKickBlack    = uint32(4)
)

func IslandAccessOp(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21302
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21303, err
	}

	response := &protobuf.SC_21303{Result: proto.Uint32(1)}
	maxCount := loadIslandSetInt("whit_list_max_cnt", 100)
	if maxCount == 0 {
		maxCount = 100
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetCommanderIslandSocialStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}

		deduped := dedupeUint32List(payload.GetUserIdList())
		switch payload.GetCmd() {
		case islandAccessOpSetWhiteList:
			if len(deduped) > int(maxCount) {
				return nil
			}
			state.WhiteList = deduped
		case islandAccessOpSetBlackList:
			if len(deduped) > int(maxCount) {
				return nil
			}
			state.BlackList = deduped
		case islandAccessOpKick:
			response.Result = proto.Uint32(0)
			return nil
		case islandAccessOpKickBlack:
			nextBlackList := dedupeUint32List(append(append([]uint32(nil), state.BlackList...), deduped...))
			if len(nextBlackList) > int(maxCount) {
				return nil
			}
			state.BlackList = nextBlackList
		default:
			return nil
		}

		if err := orm.SaveCommanderIslandSocialStateTx(context.Background(), tx, state); err != nil {
			return err
		}
		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		response.Result = proto.Uint32(1)
	}

	return client.SendMessage(21303, response)
}

func dedupeUint32List(values []uint32) []uint32 {
	if len(values) == 0 {
		return []uint32{}
	}
	seen := make(map[uint32]struct{}, len(values))
	out := make([]uint32, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
