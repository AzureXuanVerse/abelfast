package island

import (
	"fmt"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	islandSignInResultSuccess = uint32(0)
	islandSignInResultFailed  = uint32(1)
)

func IslandSignInGiftClaim(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21310
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21311, err
	}

	response := &protobuf.SC_21311{Result: proto.Uint32(islandSignInResultFailed), DropList: []*protobuf.DROPINFO{}}
	if client.Commander == nil {
		return client.SendMessage(21311, response)
	}

	state, err := orm.GetIslandSignInState(client.Commander.CommanderID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21311, response)
		}
		state = &orm.IslandSignInState{
			CommanderID:  client.Commander.CommanderID,
			ClaimedSlots: []string{},
		}
	}

	now := time.Now().UTC()
	dayStart := orm.CurrentDayResetUnix(now)
	if state.DayStartUnix != dayStart {
		state.DayStartUnix = dayStart
		state.SignedIn = false
		state.ExternalClaimCount = 0
		state.ClaimedSlots = []string{}
	}

	pos := payload.GetPos()
	if pos == 0 {
		if state.SignedIn {
			return client.SendMessage(21311, response)
		}
		state.SignedIn = true
		if err := orm.UpsertIslandSignInState(state); err != nil {
			return client.SendMessage(21311, response)
		}
		response.Result = proto.Uint32(islandSignInResultSuccess)
		return client.SendMessage(21311, response)
	}

	maxSlots := loadIslandSetInt("daily_gift_drop_num", 6)
	if pos == 0 || pos > maxSlots {
		return client.SendMessage(21311, response)
	}

	targetIslandID := payload.GetIslandId()
	if targetIslandID == 0 {
		targetIslandID = client.Commander.CommanderID
	}
	key := fmt.Sprintf("%d:%d", targetIslandID, pos)
	for _, claimed := range state.ClaimedSlots {
		if claimed == key {
			return client.SendMessage(21311, response)
		}
	}

	if targetIslandID != client.Commander.CommanderID {
		maxExternal := loadIslandSetInt("daily_gift_get_max", 3)
		if state.ExternalClaimCount >= maxExternal {
			return client.SendMessage(21311, response)
		}
	}

	if err := ensureCommanderLoaded(client, "Island/SignInGift"); err != nil {
		return client.SendMessage(21311, response)
	}

	dropID := loadIslandSetInt("daily_gift", 20001)
	drop := newDropInfo(consts.DROP_TYPE_ITEM, dropID, 1)
	if ok, err := applyDrop(client, drop.GetType(), drop.GetId(), drop.GetNumber()); err != nil || !ok {
		return client.SendMessage(21311, response)
	}

	state.ClaimedSlots = append(state.ClaimedSlots, key)
	if targetIslandID != client.Commander.CommanderID {
		state.ExternalClaimCount++
	}
	if err := orm.UpsertIslandSignInState(state); err != nil {
		return client.SendMessage(21311, response)
	}

	response.Result = proto.Uint32(islandSignInResultSuccess)
	response.DropList = []*protobuf.DROPINFO{drop}
	return client.SendMessage(21311, response)
}
