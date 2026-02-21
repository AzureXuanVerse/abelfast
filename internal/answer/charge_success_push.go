package answer

import (
	"context"
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

type ChargeSuccessEvent struct {
	ShopID  uint32
	PayID   string
	Gem     uint32
	GemFree uint32
}

func ApplyChargeSuccessEvent(commander *orm.Commander, client *connection.Client, event ChargeSuccessEvent) error {
	if commander == nil {
		return errors.New("missing commander")
	}
	if event.ShopID == 0 || event.PayID == "" {
		return errors.New("invalid charge success event")
	}
	ctx := context.Background()
	processed := false
	err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		inserted, err := orm.TryRecordChargeSuccessEventTx(ctx, tx, commander.CommanderID, event.PayID)
		if err != nil {
			return err
		}
		if !inserted {
			return nil
		}
		if event.Gem > 0 {
			if err := commander.AddResourceTx(ctx, tx, 4, event.Gem); err != nil {
				return err
			}
		}
		if event.GemFree > 0 {
			if err := commander.AddResourceTx(ctx, tx, 14, event.GemFree); err != nil {
				return err
			}
		}
		processed = true
		return nil
	})
	if err != nil {
		return err
	}
	if !processed {
		return nil
	}

	if client == nil {
		return nil
	}
	response := protobuf.SC_11503{
		ShopId:  proto.Uint32(event.ShopID),
		PayId:   proto.String(event.PayID),
		Gem:     proto.Uint32(event.Gem),
		GemFree: proto.Uint32(event.GemFree),
	}
	_, _, err = client.SendMessage(11503, &response)
	return err
}
