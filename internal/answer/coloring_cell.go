package answer

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func ColoringCell(buffer *[]byte, client *connection.Client) (int, int, error) {
	payload := &protobuf.CS_26004{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 26005, err
	}
	response := &protobuf.SC_26005{Result: proto.Uint32(coloringResultFailure)}

	pages, err := loadColoringActivityPages(payload.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26005, client, response)
	}

	pageExists := false
	for i := range pages {
		if pages[i].PageID == payload.GetId() {
			pageExists = true
			break
		}
	}
	if !pageExists {
		return connection.SendProtoMessage(26005, client, response)
	}

	template, err := loadColoringTemplate(payload.GetId())
	if err != nil {
		return connection.SendProtoMessage(26005, client, response)
	}

	cellTemplateLookup := buildColoringCellTemplateLookup(template)
	if len(cellTemplateLookup) == 0 {
		return connection.SendProtoMessage(26005, client, response)
	}

	itemConsume := make(map[uint32]uint32)
	appliedCells := make(map[string]*protobuf.CELLSINFO, len(payload.GetCellList()))
	for _, cell := range payload.GetCellList() {
		key := coloringCellKey(cell.GetRow(), cell.GetColumn())
		tplCell, ok := cellTemplateLookup[key]
		if !ok {
			return connection.SendProtoMessage(26005, client, response)
		}
		color := cell.GetColor()
		if color > uint32(len(template.ColorIDList)) {
			return connection.SendProtoMessage(26005, client, response)
		}

		if template.Blank == 0 {
			if color == 0 || tplCell.Required == 0 || color != tplCell.Required {
				return connection.SendProtoMessage(26005, client, response)
			}
		}

		appliedCells[key] = cell
	}

	if template.Blank == 0 {
		for _, cell := range appliedCells {
			itemID := template.ColorIDList[cell.GetColor()-1]
			if itemID == 0 {
				return connection.SendProtoMessage(26005, client, response)
			}
			itemConsume[itemID] += 1
		}
	}

	state, err := getOrCreateColoringState(client.Commander.CommanderID, payload.GetActId())
	if err != nil {
		return connection.SendProtoMessage(26005, client, response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		for itemID, count := range itemConsume {
			ok, consumeErr := consumeCommanderItemTx(context.Background(), tx, client.Commander.CommanderID, itemID, count)
			if consumeErr != nil {
				return consumeErr
			}
			if !ok {
				return errColoringInsufficientPaint
			}
		}

		for _, cell := range appliedCells {
			coloringSetCell(state, payload.GetId(), cell.GetRow(), cell.GetColumn(), cell.GetColor())
		}
		return orm.SaveCommanderColoringStateTx(context.Background(), tx, state)
	})
	if err != nil {
		return connection.SendProtoMessage(26005, client, response)
	}

	if err := client.Commander.Load(); err != nil {
		return connection.SendProtoMessage(26005, client, response)
	}

	response.Result = proto.Uint32(coloringResultSuccess)
	return connection.SendProtoMessage(26005, client, response)
}
