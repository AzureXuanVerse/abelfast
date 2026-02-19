package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

const (
	metaPtClaimResultSuccess      = 0
	metaPtClaimResultInvalidGroup = 1
	metaPtClaimResultInvalidTier  = 2
	metaPtClaimResultInsufficient = 3
	metaPtClaimResultClaimed      = 4
)

var (
	errMetaPtAlreadyClaimed = errors.New("meta pt threshold already claimed")
	errMetaPtInsufficient   = errors.New("meta pt insufficient")
)

var metaPtConfigCategories = []string{
	"ShareCfg/ship_strengthen_meta.json",
	"sharecfgdata/ship_strengthen_meta.json",
}

type metaPtConfig struct {
	ID           uint32     `json:"id"`
	Type         uint32     `json:"type"`
	Target       []uint32   `json:"target"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

func ClaimMetaPtAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	payload := &protobuf.CS_34003{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 34004, err
	}

	response := &protobuf.SC_34004{Result: proto.Uint32(metaPtClaimResultInvalidGroup), DropList: []*protobuf.DROPINFO{}}
	config, err := loadMetaPtConfig(payload.GetGroupId())
	if err != nil || config == nil || config.Type != 1 {
		return connection.SendProtoMessage(34004, client, response)
	}

	tierIndex := metaPtTierIndex(config.Target, payload.GetTargetPt())
	if tierIndex < 0 {
		response.Result = proto.Uint32(metaPtClaimResultInvalidTier)
		return connection.SendProtoMessage(34004, client, response)
	}

	drops, ok := buildMetaPtTierDrops(config.AwardDisplay, tierIndex)
	if !ok {
		response.Result = proto.Uint32(metaPtClaimResultInvalidTier)
		return connection.SendProtoMessage(34004, client, response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		progress, txErr := orm.GetOrCreateCommanderMetaPtProgressTx(context.Background(), tx, client.Commander.CommanderID, payload.GetGroupId())
		if txErr != nil {
			return txErr
		}
		if progress.Pt < payload.GetTargetPt() {
			return errMetaPtInsufficient
		}
		if metaPtContains(progress.FetchList, payload.GetTargetPt()) {
			return errMetaPtAlreadyClaimed
		}
		if txErr := applyLoveLetterDropsTx(context.Background(), tx, client, drops); txErr != nil {
			return txErr
		}
		progress.FetchList = append(progress.FetchList, payload.GetTargetPt())
		sort.Slice(progress.FetchList, func(i int, j int) bool {
			return progress.FetchList[i] < progress.FetchList[j]
		})
		return orm.SaveCommanderMetaPtProgressTx(context.Background(), tx, progress)
	})
	if err != nil {
		switch {
		case errors.Is(err, errMetaPtInsufficient):
			response.Result = proto.Uint32(metaPtClaimResultInsufficient)
		case errors.Is(err, errMetaPtAlreadyClaimed):
			response.Result = proto.Uint32(metaPtClaimResultClaimed)
		default:
			response.Result = proto.Uint32(metaPtClaimResultInvalidTier)
		}
		return connection.SendProtoMessage(34004, client, response)
	}

	response.Result = proto.Uint32(metaPtClaimResultSuccess)
	response.DropList = dropMapToSortedList(drops)
	return connection.SendProtoMessage(34004, client, response)
}

func loadMetaPtConfig(groupID uint32) (*metaPtConfig, error) {
	key := fmt.Sprintf("%d", groupID)
	for _, category := range metaPtConfigCategories {
		entry, err := orm.GetConfigEntry(category, key)
		if err == nil {
			config := &metaPtConfig{}
			if unmarshalErr := json.Unmarshal(entry.Data, config); unmarshalErr != nil {
				return nil, unmarshalErr
			}
			return config, nil
		}
		if !errors.Is(err, db.ErrNotFound) {
			return nil, err
		}
	}
	return nil, nil
}

func metaPtTierIndex(targets []uint32, targetPt uint32) int {
	for i, value := range targets {
		if value == targetPt {
			return i
		}
	}
	return -1
}

func buildMetaPtTierDrops(awardDisplay [][]uint32, tierIndex int) (map[string]*protobuf.DROPINFO, bool) {
	if tierIndex < 0 || tierIndex >= len(awardDisplay) {
		return nil, false
	}
	tier := awardDisplay[tierIndex]
	if len(tier) < 3 {
		return nil, false
	}
	drops := make(map[string]*protobuf.DROPINFO, 1)
	accumulateDrop(drops, normalizeDropType(tier[0]), tier[1], tier[2])
	return drops, true
}

func normalizeDropType(dropType uint32) uint32 {
	if dropType == 2 {
		return consts.DROP_TYPE_ITEM
	}
	return dropType
}

func metaPtContains(values []uint32, value uint32) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}
