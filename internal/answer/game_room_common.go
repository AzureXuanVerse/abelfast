package answer

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ggmolly/belfast/internal/orm"
)

const (
	gameRoomCoinResourceID   = uint32(11)
	gameRoomTicketResourceID = uint32(12)
)

type gameRoomTemplate struct {
	ID      uint32          `json:"id"`
	AddBase uint32          `json:"add_base"`
	AddNum  [][]float64     `json:"add_num"`
	AddType uint32          `json:"add_type"`
	CoinMax uint32          `json:"coin_max"`
	RawHelp json.RawMessage `json:"game_help"`
	Extra   map[string]any  `json:"-"`
}

type gameRoomGamesetEntry struct {
	Description json.RawMessage `json:"description"`
	KeyValue    uint32          `json:"key_value"`
}

type gameRoomPriceTier struct {
	Threshold uint32
	Price     uint32
}

type gameRoomSettings struct {
	CoinInitial      uint32
	CoinMax          uint32
	TicketMonthlyMax uint32
	TicketTotalMax   uint32
	CoinGoldTiers    []gameRoomPriceTier
}

func loadGameRoomTemplates() ([]gameRoomTemplate, error) {
	entries, err := orm.ListConfigEntries("ShareCfg/game_room_template.json")
	if err != nil {
		return nil, err
	}
	templates := make([]gameRoomTemplate, 0, len(entries))
	for _, entry := range entries {
		var template gameRoomTemplate
		if err := json.Unmarshal(entry.Data, &template); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].ID < templates[j].ID
	})
	return templates, nil
}

func loadGameRoomTemplate(roomID uint32) (*gameRoomTemplate, bool, error) {
	templates, err := loadGameRoomTemplates()
	if err != nil {
		return nil, false, err
	}
	for _, template := range templates {
		if template.ID == roomID {
			matched := template
			return &matched, true, nil
		}
	}
	return nil, false, nil
}

func loadGameRoomSettings() (*gameRoomSettings, error) {
	coinInitial, err := loadGameRoomGamesetValue("game_coin_initial")
	if err != nil {
		return nil, err
	}
	coinMax, err := loadGameRoomGamesetValue("game_coin_max")
	if err != nil {
		return nil, err
	}
	ticketMonthlyMax, err := loadGameRoomGamesetValue("game_ticket_month")
	if err != nil {
		return nil, err
	}
	ticketTotalMax, err := loadGameRoomGamesetValue("game_room_remax")
	if err != nil {
		return nil, err
	}
	coinGoldEntry, err := orm.GetConfigEntry("ShareCfg/gameset.json", "game_coin_gold")
	if err != nil {
		return nil, err
	}
	var coinGold gameRoomGamesetEntry
	if err := json.Unmarshal(coinGoldEntry.Data, &coinGold); err != nil {
		return nil, err
	}
	tiers, err := parseGameRoomCoinGoldTiers(coinGold.Description)
	if err != nil {
		return nil, err
	}

	return &gameRoomSettings{
		CoinInitial:      coinInitial,
		CoinMax:          coinMax,
		TicketMonthlyMax: ticketMonthlyMax,
		TicketTotalMax:   ticketTotalMax,
		CoinGoldTiers:    tiers,
	}, nil
}

func loadGameRoomGamesetValue(key string) (uint32, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/gameset.json", key)
	if err != nil {
		return 0, err
	}
	var data gameRoomGamesetEntry
	if err := json.Unmarshal(entry.Data, &data); err != nil {
		return 0, err
	}
	return data.KeyValue, nil
}

func parseGameRoomCoinGoldTiers(raw json.RawMessage) ([]gameRoomPriceTier, error) {
	var encoded [][]uint32
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, err
	}
	tiers := make([]gameRoomPriceTier, 0, len(encoded))
	for _, entry := range encoded {
		if len(entry) != 2 {
			return nil, fmt.Errorf("invalid game_coin_gold tier")
		}
		tiers = append(tiers, gameRoomPriceTier{Threshold: entry[0], Price: entry[1]})
	}
	sort.Slice(tiers, func(i, j int) bool {
		return tiers[i].Threshold < tiers[j].Threshold
	})
	return tiers, nil
}

func gameRoomExchangePriceByCount(tiers []gameRoomPriceTier, count uint32) uint32 {
	if len(tiers) == 0 {
		return 0
	}
	price := tiers[0].Price
	for _, tier := range tiers {
		if count >= tier.Threshold {
			price = tier.Price
		}
	}
	return price
}

func gameRoomMultiplierForScore(thresholds [][]float64, score uint32) float64 {
	if len(thresholds) == 0 {
		return 0
	}
	bestThreshold := uint32(0)
	multiplier := thresholds[0][1]
	for _, entry := range thresholds {
		if len(entry) != 2 {
			continue
		}
		threshold := uint32(entry[0])
		if score >= threshold && threshold >= bestThreshold {
			bestThreshold = threshold
			multiplier = entry[1]
		}
	}
	return multiplier
}

func isNotEnoughResourcesError(err error) bool {
	return err != nil && err.Error() == "not enough resources"
}
