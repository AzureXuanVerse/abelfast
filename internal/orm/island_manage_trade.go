package orm

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

type IslandManageTrade struct {
	CommanderID uint32 `gorm:"primaryKey;column:commander_id"`
	TradeID     uint32 `gorm:"primaryKey;column:trade_id"`
	TradeData   []byte `gorm:"column:trade_data"`
	PresellData []byte `gorm:"column:presell_data"`
	TotalSales  uint32 `gorm:"column:total_sales"`
}

func (IslandManageTrade) TableName() string {
	return "island_manage_trades"
}

func UpsertIslandManageTradeTx(ctx context.Context, tx pgx.Tx, commanderID uint32, trade *protobuf.PB_ISLAND_TRADE, presell *protobuf.PB_TRADE_PRESELL, totalSales uint32) error {
	tradeData, err := proto.Marshal(trade)
	if err != nil {
		return err
	}
	presellData, err := proto.Marshal(presell)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO island_manage_trades (commander_id, trade_id, trade_data, presell_data, total_sales)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (commander_id, trade_id)
DO UPDATE SET
	trade_data = EXCLUDED.trade_data,
	presell_data = EXCLUDED.presell_data,
	total_sales = EXCLUDED.total_sales
`, int64(commanderID), int64(trade.GetId()), tradeData, presellData, int64(totalSales))
	return err
}

func GetIslandManageTradeForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32, tradeID uint32) (*protobuf.PB_ISLAND_TRADE, *protobuf.PB_TRADE_PRESELL, uint32, error) {
	var tradeData []byte
	var presellData []byte
	var totalSalesRaw int64
	err := tx.QueryRow(ctx, `
SELECT trade_data, presell_data, total_sales
FROM island_manage_trades
WHERE commander_id = $1 AND trade_id = $2
FOR UPDATE
`, int64(commanderID), int64(tradeID)).Scan(&tradeData, &presellData, &totalSalesRaw)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, nil, 0, err
	}

	trade := &protobuf.PB_ISLAND_TRADE{}
	if err := proto.Unmarshal(tradeData, trade); err != nil {
		return nil, nil, 0, err
	}
	presell := &protobuf.PB_TRADE_PRESELL{}
	if err := proto.Unmarshal(presellData, presell); err != nil {
		return nil, nil, 0, err
	}
	return trade, presell, uint32(totalSalesRaw), nil
}
