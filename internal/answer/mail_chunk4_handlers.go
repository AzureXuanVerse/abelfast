package answer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

const (
	mailChunkResultSuccess = uint32(0)
	mailChunkResultFailed  = uint32(1)

	mailStoreroomGoldResourceID      = uint32(1)
	mailStoreroomOilResourceID       = uint32(2)
	mailStoreroomGemResourceID       = uint32(4)
	mailStoreroomStoredGoldResource  = uint32(16)
	mailStoreroomStoredOilResource   = uint32(17)
	mailStoreroomConfigEntryCategory = "ShareCfg/mail_storeroom.json"

	loveLetterRepairResultInvalid      = uint32(6)
	loveLetterRepairResultMissingItem  = uint32(7)
	loveLetterRepairResultInventoryCap = uint32(40)
)

type mailStoreroomConfig struct {
	Level       uint32 `json:"level"`
	UpgradeGem  uint32 `json:"upgrade_gem"`
	UpgradeGold uint32 `json:"upgrade_gold"`
	OilStore    uint32 `json:"oil_store"`
	GoldStore   uint32 `json:"gold_store"`
}

func ExtendMailStoreroomCapacity(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_30010
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 30010, err
	}

	response := &protobuf.SC_30011{Result: proto.Uint32(mailChunkResultFailed)}
	if err := ensureCommanderLoaded(client, "Mail/StoreroomExtend"); err != nil {
		return connection.SendProtoMessage(30011, client, response)
	}

	configByLevel, err := loadMailStoreroomConfigByLevel()
	if err != nil {
		return connection.SendProtoMessage(30011, client, response)
	}

	currentLevel := client.Commander.MailStoreroomLv
	if currentLevel == 0 {
		currentLevel = 1
	}
	currentConfig, ok := configByLevel[currentLevel]
	if !ok {
		return connection.SendProtoMessage(30011, client, response)
	}
	if _, hasNext := configByLevel[currentLevel+1]; !hasNext {
		return connection.SendProtoMessage(30011, client, response)
	}

	resourceID, cost, ok := resolveMailStoreroomUpgradeCost(payload.GetArg(), currentConfig)
	if !ok || !client.Commander.HasEnoughResource(resourceID, cost) {
		return connection.SendProtoMessage(30011, client, response)
	}

	previousLevel := client.Commander.MailStoreroomLv
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		if err := client.Commander.ConsumeResourceTx(ctx, tx, resourceID, cost); err != nil {
			return err
		}
		client.Commander.MailStoreroomLv = currentLevel + 1
		return client.Commander.SaveTx(ctx, tx)
	})
	if err != nil {
		client.Commander.MailStoreroomLv = previousLevel
		_ = client.Commander.Load()
		return connection.SendProtoMessage(30011, client, response)
	}

	response.Result = proto.Uint32(mailChunkResultSuccess)
	return connection.SendProtoMessage(30011, client, response)
}

func WithdrawMailStoreroomResources(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_30012
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 30012, err
	}

	response := &protobuf.SC_30013{Result: proto.Uint32(mailChunkResultFailed)}
	if err := ensureCommanderLoaded(client, "Mail/StoreroomWithdraw"); err != nil {
		return connection.SendProtoMessage(30013, client, response)
	}

	oil := payload.GetOil()
	gold := payload.GetGold()
	if oil == 0 && gold == 0 {
		return connection.SendProtoMessage(30013, client, response)
	}
	if (oil > 0 && !client.Commander.HasEnoughResource(mailStoreroomStoredOilResource, oil)) ||
		(gold > 0 && !client.Commander.HasEnoughResource(mailStoreroomStoredGoldResource, gold)) {
		return connection.SendProtoMessage(30013, client, response)
	}

	maxOil, maxGold, err := loadMailStoreroomWithdrawCaps()
	if err != nil {
		return connection.SendProtoMessage(30013, client, response)
	}
	if maxOil > 0 && client.Commander.GetResourceCount(mailStoreroomOilResourceID)+oil > maxOil {
		return connection.SendProtoMessage(30013, client, response)
	}
	if maxGold > 0 && client.Commander.GetResourceCount(mailStoreroomGoldResourceID)+gold > maxGold {
		return connection.SendProtoMessage(30013, client, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		if oil > 0 {
			if err := client.Commander.ConsumeResourceTx(ctx, tx, mailStoreroomStoredOilResource, oil); err != nil {
				return err
			}
			if err := client.Commander.AddResourceTx(ctx, tx, mailStoreroomOilResourceID, oil); err != nil {
				return err
			}
		}
		if gold > 0 {
			if err := client.Commander.ConsumeResourceTx(ctx, tx, mailStoreroomStoredGoldResource, gold); err != nil {
				return err
			}
			if err := client.Commander.AddResourceTx(ctx, tx, mailStoreroomGoldResourceID, gold); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
		return connection.SendProtoMessage(30013, client, response)
	}

	response.Result = proto.Uint32(mailChunkResultSuccess)
	return connection.SendProtoMessage(30013, client, response)
}

func GetMailTitleList(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_30014
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 30015, err
	}
	if err := syncCommanderMailState(client); err != nil {
		return 0, 30015, err
	}

	response := &protobuf.SC_30015{}
	for _, id := range payload.GetIdList() {
		mail := client.Commander.MailsMap[id]
		if mail == nil {
			continue
		}
		response.MailTitleList = append(response.MailTitleList, &protobuf.MAIL_TITLE{
			Id:    proto.Uint32(mail.ID),
			Title: proto.String(mailTitleWithSender(mail)),
		})
	}

	return connection.SendProtoMessage(30015, client, response)
}

func CheckLoveLetterItemMail(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_30016
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 30016, err
	}

	response := &protobuf.SC_30017{}
	bundle, err := loadLoveLetterConfigBundle()
	if err != nil {
		return connection.SendProtoMessage(30017, client, response)
	}
	yearMap := bundle.ItemGroupToYears[itemGroupKey(payload.GetItemId(), payload.GetGroupid())]
	if len(yearMap) == 0 {
		return connection.SendProtoMessage(30017, client, response)
	}

	years := make([]uint32, 1, len(yearMap)+1)
	for year := range yearMap {
		years = append(years, year)
	}
	sort.Slice(years[1:], func(i int, j int) bool {
		return years[i+1] < years[j+1]
	})
	response.Years = years
	return connection.SendProtoMessage(30017, client, response)
}

func RepairLoveLetterItemMail(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_30018
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 30018, err
	}

	response := &protobuf.SC_30019{Ret: proto.Uint32(loveLetterRepairResultInvalid)}
	if err := ensureCommanderLoaded(client, "LoveLetter/RepairItemMail"); err != nil {
		return connection.SendProtoMessage(30019, client, response)
	}
	bundle, err := loadLoveLetterConfigBundle()
	if err != nil {
		return connection.SendProtoMessage(30019, client, response)
	}

	_, _, letterID, ok := resolveLoveLetterRepairTarget(
		bundle,
		payload.GetItemId(),
		payload.GetGroupid(),
		payload.GetYear(),
	)
	if !ok {
		return connection.SendProtoMessage(30019, client, response)
	}

	dropMap := make(map[string]*protobuf.DROPINFO)
	accumulateDrop(dropMap, consts.DROP_TYPE_LOVE_LETTER, letterID, 1)

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ctx := context.Background()
		if err := client.Commander.ConsumeItemTx(ctx, tx, payload.GetItemId(), 1); err != nil {
			return err
		}
		return applyLoveLetterDropsTx(ctx, tx, client, dropMap)
	})
	if err != nil {
		response.Ret = proto.Uint32(resolveLoveLetterRepairFailure(err))
		_ = client.Commander.Load()
		return connection.SendProtoMessage(30019, client, response)
	}

	response.Ret = proto.Uint32(mailChunkResultSuccess)
	response.DropList = dropMapToSortedList(dropMap)
	return connection.SendProtoMessage(30019, client, response)
}

func loadMailStoreroomConfigByLevel() (map[uint32]mailStoreroomConfig, error) {
	entries, err := orm.ListConfigEntries(mailStoreroomConfigEntryCategory)
	if err != nil {
		return nil, err
	}
	configs := make(map[uint32]mailStoreroomConfig, len(entries))
	for _, entry := range entries {
		if !isJSONMap(entry.Data) {
			continue
		}
		var cfg mailStoreroomConfig
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			return nil, err
		}
		if cfg.Level == 0 {
			continue
		}
		configs[cfg.Level] = cfg
	}
	return configs, nil
}

func resolveMailStoreroomUpgradeCost(arg uint32, cfg mailStoreroomConfig) (uint32, uint32, bool) {
	switch arg {
	case mailStoreroomGoldResourceID:
		if cfg.UpgradeGold == 0 {
			return 0, 0, false
		}
		return mailStoreroomGoldResourceID, cfg.UpgradeGold, true
	case mailStoreroomGemResourceID:
		if cfg.UpgradeGem == 0 {
			return 0, 0, false
		}
		return mailStoreroomGemResourceID, cfg.UpgradeGem, true
	default:
		return 0, 0, false
	}
}

func loadMailStoreroomWithdrawCaps() (uint32, uint32, error) {
	maxOilEntry, err := loadGameSetEntry("max_oil")
	if err != nil {
		return 0, 0, err
	}
	maxGoldEntry, err := loadGameSetEntry("max_gold")
	if err != nil {
		return 0, 0, err
	}
	return maxOilEntry.KeyValue, maxGoldEntry.KeyValue, nil
}

func resolveLoveLetterRepairTarget(
	bundle *loveLetterConfigBundle,
	itemID uint32,
	groupID uint32,
	year uint32,
) (uint32, uint32, uint32, bool) {
	candidates := make(map[uint32]uint32)
	if groupID > 0 {
		for candidateYear, canonicalGroup := range bundle.ItemGroupToYears[itemGroupKey(itemID, groupID)] {
			candidates[candidateYear] = canonicalGroup
		}
	} else {
		prefix := fmt.Sprintf("%d_", itemID)
		for key, yearMap := range bundle.ItemGroupToYears {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			for candidateYear, canonicalGroup := range yearMap {
				existing, exists := candidates[candidateYear]
				if exists && existing != canonicalGroup {
					candidates[candidateYear] = 0
					continue
				}
				candidates[candidateYear] = canonicalGroup
			}
		}
	}
	if len(candidates) == 0 {
		return 0, 0, 0, false
	}

	if year > 0 {
		canonicalGroup, ok := candidates[year]
		if !ok || canonicalGroup == 0 {
			return 0, 0, 0, false
		}
		letterID := bundle.LetterByGroupYear[groupYearKey(canonicalGroup, year)]
		if letterID == 0 {
			return 0, 0, 0, false
		}
		return year, canonicalGroup, letterID, true
	}

	var selectedYear uint32
	var selectedGroup uint32
	for candidateYear, canonicalGroup := range candidates {
		if canonicalGroup == 0 {
			continue
		}
		if bundle.LetterByGroupYear[groupYearKey(canonicalGroup, candidateYear)] == 0 {
			continue
		}
		if selectedYear != 0 {
			return 0, 0, 0, false
		}
		selectedYear = candidateYear
		selectedGroup = canonicalGroup
	}
	if selectedYear == 0 {
		return 0, 0, 0, false
	}
	letterID := bundle.LetterByGroupYear[groupYearKey(selectedGroup, selectedYear)]
	if letterID == 0 {
		return 0, 0, 0, false
	}
	return selectedYear, selectedGroup, letterID, true
}

func resolveLoveLetterRepairFailure(err error) uint32 {
	if err == nil {
		return mailChunkResultSuccess
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "not enough items") {
		return loveLetterRepairResultMissingItem
	}
	return loveLetterRepairResultInventoryCap
}
