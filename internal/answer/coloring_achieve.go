package answer

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func ColoringAchieve(buffer *[]byte, client *connection.Client) (int, int, error) {
	payload := &protobuf.CS_26002{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 26003, err
	}
	response := &protobuf.SC_26003{Result: proto.Uint32(coloringResultFailure), DropList: []*protobuf.DROPINFO{}}

	pages, err := loadColoringActivityPages(payload.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26003, client, response)
	}

	var page *coloringActivityPage
	for i := range pages {
		if pages[i].PageID == payload.GetId() {
			page = &pages[i]
			break
		}
	}
	if page == nil {
		return connection.SendProtoMessage(26003, client, response)
	}

	template, err := loadColoringTemplate(page.PageID)
	if err != nil {
		return connection.SendProtoMessage(26003, client, response)
	}

	state, err := getOrCreateColoringState(client.Commander.CommanderID, payload.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26003, client, response)
	}
	if coloringIsPageClaimed(state, page.PageID) {
		return connection.SendProtoMessage(26003, client, response)
	}

	fills := coloringGetPageFills(state, page.PageID)
	if !coloringIsPageComplete(template, fills) {
		return connection.SendProtoMessage(26003, client, response)
	}

	drops := coloringResolveClaimDrops(page.RewardSpec)

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		coloringAddClaim(state, page.PageID, drops)
		if err := orm.SaveCommanderColoringStateTx(context.Background(), tx, state); err != nil {
			return err
		}
		return coloringApplyDropsTx(context.Background(), tx, client.Commander.CommanderID, drops)
	})
	if err != nil {
		return connection.SendProtoMessage(26003, client, response)
	}

	if err := client.Commander.Load(); err != nil {
		return connection.SendProtoMessage(26003, client, response)
	}

	response.Result = proto.Uint32(coloringResultSuccess)
	response.DropList = drops
	return connection.SendProtoMessage(26003, client, response)
}
