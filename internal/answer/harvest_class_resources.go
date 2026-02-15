package answer

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func HarvestClassResource(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_22009
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 22010, err
	}

	if client.Commander.OwnedResourcesMap == nil || client.Commander.CommanderItemsMap == nil {
		if err := client.Commander.Load(); err != nil {
			return 0, 22010, err
		}
	}

	expInWell := client.Commander.GetResourceCount(classFieldResourceID)
	response := protobuf.SC_22010{
		Result:    proto.Uint32(1),
		ExpInWell: proto.Uint32(expInWell),
	}

	if payload.GetType() != 0 {
		return client.SendMessage(22010, &response)
	}

	classItemID, err := loadClassResourceItemID()
	if err != nil {
		return client.SendMessage(22010, &response)
	}
	itemConfig, err := loadItemStatisticsConfig(classItemID)
	if err != nil {
		return client.SendMessage(22010, &response)
	}
	packExpValue, err := parseUsageArgExpValue(itemConfig.UsageArg)
	if err != nil || packExpValue == 0 || itemConfig.MaxNum == 0 {
		return client.SendMessage(22010, &response)
	}

	generated := expInWell / packExpValue
	currentCount := client.Commander.GetItemCount(classItemID)
	freeCapacity := uint32(0)
	if currentCount < itemConfig.MaxNum {
		freeCapacity = itemConfig.MaxNum - currentCount
	}
	claimCount := generated
	if claimCount > freeCapacity {
		claimCount = freeCapacity
	}
	if claimCount == 0 {
		return client.SendMessage(22010, &response)
	}

	consumedExp := claimCount * packExpValue
	newExpInWell := expInWell - consumedExp

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if err := client.Commander.AddItemTx(ctx, tx, classItemID, claimCount); err != nil {
			return err
		}
		if err := client.Commander.ConsumeResourceTx(ctx, tx, classFieldResourceID, consumedExp); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, 22010, err
	}

	response.Result = proto.Uint32(0)
	response.ExpInWell = proto.Uint32(newExpInWell)
	return client.SendMessage(22010, &response)
}
