package island

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	islandCharaTemplateCategory       = "ShareCfg/island_chara_template.json"
	islandCharaTemplateCategoryLC     = "sharecfgdata/island_chara_template.json"
	islandCharaSkillCategory          = "ShareCfg/island_chara_skill.json"
	islandCharaSkillCategoryLC        = "sharecfgdata/island_chara_skill.json"
	islandCharaLevelCategory          = "ShareCfg/island_chara_level.json"
	islandCharaLevelCategoryLC        = "sharecfgdata/island_chara_level.json"
	islandDressColorTemplateCategory  = "ShareCfg/island_dress_colordiff_template.json"
	islandDressColorTemplateCategoryL = "sharecfgdata/island_dress_colordiff_template.json"
	islandSkinColorTemplateCategory   = "ShareCfg/island_skin_colordiff_template.json"
	islandSkinColorTemplateCategoryLC = "sharecfgdata/island_skin_colordiff_template.json"
	islandSkinTemplateCategory        = "ShareCfg/island_skin_template.json"
	islandSkinTemplateCategoryLC      = "sharecfgdata/island_skin_template.json"
	islandActionFeedbackCategory      = "ShareCfg/island_action_feedback.json"
	islandActionFeedbackCategoryLC    = "sharecfgdata/island_action_feedback.json"
	islandStrollNPCCategory           = "ShareCfg/island_strollnpc.json"
	islandStrollNPCCategoryLC         = "sharecfgdata/island_strollnpc.json"
	islandBuffTemplateCategory        = "ShareCfg/island_buff_template.json"
	islandBuffTemplateCategoryLC      = "sharecfgdata/island_buff_template.json"
	islandDropDataTemplateCategory    = "ShareCfg/drop_data_template.json"
	islandDropDataTemplateCategoryLC  = "sharecfgdata/drop_data_template.json"
)

type islandCharaTemplateConfig struct {
	ID          uint32          `json:"id"`
	Power       uint32          `json:"power"`
	SkillID     uint32          `json:"skill_id"`
	SkillUnlock uint32          `json:"skill_unlock"`
	AttItem     json.RawMessage `json:"att_item"`
	ExtraMax    json.RawMessage `json:"extra_max"`
	Favorite    json.RawMessage `json:"favorite_gift"`
}

type islandCharaSkillConfig struct {
	ID          uint32          `json:"id"`
	Material    json.RawMessage `json:"material"`
	SkillEffect json.RawMessage `json:"skill_effect"`
}

type islandCharaLevelConfig struct {
	ID         uint32 `json:"id"`
	LevelUpExp uint32 `json:"level_up_exp"`
}

type islandDressColorConfig struct {
	ID            uint32     `json:"id"`
	BelongToDress uint32     `json:"belongto_dress"`
	Cost          [][]uint32 `json:"cost"`
}

type islandSkinColorConfig struct {
	ID        uint32     `json:"id"`
	SkinGroup uint32     `json:"skin_group"`
	Cost      [][]uint32 `json:"cost"`
}

type islandSkinConfig struct {
	ID        uint32 `json:"id"`
	ShipGroup uint32 `json:"ship_group"`
}

type islandActionFeedbackConfig struct {
	ID     uint32 `json:"id"`
	DropID uint32 `json:"drop_id"`
}

type islandStrollNPCConfig struct {
	ID             uint32 `json:"id"`
	ActionFeedback uint32 `json:"action_feedback"`
}

type islandBuffTemplateConfig struct {
	ID       uint32 `json:"id"`
	DuelType uint32 `json:"duel_type"`
	DuelID   uint32 `json:"duel_id"`
}

type islandDropDataConfig struct {
	ID       uint32          `json:"id"`
	DropList json.RawMessage `json:"drop_list"`
	Display  json.RawMessage `json:"display"`
	Award    json.RawMessage `json:"award_display"`
}

func loadIslandConfigByID[T any](id uint32, categories ...string) (*T, bool, error) {
	key := strconv.FormatUint(uint64(id), 10)
	for _, category := range categories {
		if entry, err := orm.GetConfigEntry(category, key); err == nil {
			var cfg T
			if err := json.Unmarshal(entry.Data, &cfg); err == nil {
				return &cfg, true, nil
			}
		}
	}

	for _, category := range categories {
		entries, err := orm.ListConfigEntries(category)
		if err != nil {
			continue
		}
		for i := range entries {
			var single T
			if err := json.Unmarshal(entries[i].Data, &single); err == nil {
				if parseConfigID(single) == id {
					return &single, true, nil
				}
			}
			var list []T
			if err := json.Unmarshal(entries[i].Data, &list); err != nil {
				continue
			}
			for j := range list {
				if parseConfigID(list[j]) == id {
					return &list[j], true, nil
				}
			}
		}
	}
	return nil, false, nil
}

func parseConfigID(v any) uint32 {
	raw, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	var row struct {
		ID uint32 `json:"id"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return 0
	}
	return row.ID
}

func loadIslandCharaTemplate(shipID uint32) (*islandCharaTemplateConfig, bool, error) {
	return loadIslandConfigByID[islandCharaTemplateConfig](shipID, islandCharaTemplateCategory, islandCharaTemplateCategoryLC)
}

func loadIslandCharaSkill(skillID uint32) (*islandCharaSkillConfig, bool, error) {
	return loadIslandConfigByID[islandCharaSkillConfig](skillID, islandCharaSkillCategory, islandCharaSkillCategoryLC)
}

func loadIslandCharaLevel(level uint32) (*islandCharaLevelConfig, bool, error) {
	return loadIslandConfigByID[islandCharaLevelConfig](level, islandCharaLevelCategory, islandCharaLevelCategoryLC)
}

func loadIslandDressColorConfig(colorID uint32) (*islandDressColorConfig, bool, error) {
	return loadIslandConfigByID[islandDressColorConfig](colorID, islandDressColorTemplateCategory, islandDressColorTemplateCategoryL)
}

func loadIslandSkinColorConfig(colorID uint32) (*islandSkinColorConfig, bool, error) {
	return loadIslandConfigByID[islandSkinColorConfig](colorID, islandSkinColorTemplateCategory, islandSkinColorTemplateCategoryLC)
}

func loadIslandSkinConfig(skinID uint32) (*islandSkinConfig, bool, error) {
	return loadIslandConfigByID[islandSkinConfig](skinID, islandSkinTemplateCategory, islandSkinTemplateCategoryLC)
}

func loadIslandActionFeedbackConfig(feedbackID uint32) (*islandActionFeedbackConfig, bool, error) {
	return loadIslandConfigByID[islandActionFeedbackConfig](feedbackID, islandActionFeedbackCategory, islandActionFeedbackCategoryLC)
}

func loadIslandStrollNPCConfig(npcID uint32) (*islandStrollNPCConfig, bool, error) {
	return loadIslandConfigByID[islandStrollNPCConfig](npcID, islandStrollNPCCategory, islandStrollNPCCategoryLC)
}

func loadIslandBuffTemplateConfig(buffID uint32) (*islandBuffTemplateConfig, bool, error) {
	return loadIslandConfigByID[islandBuffTemplateConfig](buffID, islandBuffTemplateCategory, islandBuffTemplateCategoryLC)
}

func loadIslandDropDataConfig(dropID uint32) (*islandDropDataConfig, bool, error) {
	return loadIslandConfigByID[islandDropDataConfig](dropID, islandDropDataTemplateCategory, islandDropDataTemplateCategoryLC)
}

func buildIslandShipProto(ship *orm.IslandShip) *protobuf.PB_ISLAND_SHIP {
	attrs := make([]*protobuf.PB_SHIP_ATTR, 0, len(ship.ExtraAttrs))
	for i := range ship.ExtraAttrs {
		attrs = append(attrs, &protobuf.PB_SHIP_ATTR{Id: proto.Uint32(ship.ExtraAttrs[i].ID), Value: proto.Uint32(ship.ExtraAttrs[i].Value)})
	}
	buffs := make([]*protobuf.PB_ISLAND_BUFF, 0, len(ship.Buffs))
	for i := range ship.Buffs {
		buffs = append(buffs, &protobuf.PB_ISLAND_BUFF{Id: proto.Uint32(ship.Buffs[i].ID), StartTime: proto.Uint32(ship.Buffs[i].StartTime)})
	}
	return &protobuf.PB_ISLAND_SHIP{
		Id:            proto.Uint32(ship.ShipID),
		Lv:            proto.Uint32(maxUint32(ship.Level, 1)),
		Exp:           proto.Uint32(ship.Exp),
		BreakLv:       proto.Uint32(maxUint32(ship.BreakLv, 1)),
		SkillLv:       proto.Uint32(maxUint32(ship.SkillLv, 1)),
		Power:         proto.Uint32(ship.Power),
		RecoverTime:   proto.Uint32(ship.RecoverTime),
		BuffList:      buffs,
		ExtraAttrList: attrs,
		UpLimitState:  proto.Uint32(ship.UpLimitState),
		CurSkinId:     proto.Uint32(ship.CurSkinID),
		WorkPlace:     &protobuf.PB_SHIP_WORK_PLACE{Type: proto.Uint32(0), Place: proto.Uint32(0)},
	}
}

func extractAttrItemMap(raw json.RawMessage) map[uint32]map[uint32]struct{} {
	out := make(map[uint32]map[uint32]struct{})
	if len(raw) == 0 {
		return out
	}
	var list [][][]uint32
	if err := decodeUsageArg(raw, &list); err == nil {
		for i := range list {
			attrType := uint32(i + 1)
			set := make(map[uint32]struct{})
			for j := range list[i] {
				if len(list[i][j]) > 0 {
					set[list[i][j][0]] = struct{}{}
				}
			}
			out[attrType] = set
		}
		return out
	}

	var obj map[string][][]uint32
	if err := decodeUsageArg(raw, &obj); err == nil {
		for key, values := range obj {
			attrType := parseUint32Key(key)
			if attrType == 0 {
				continue
			}
			set := make(map[uint32]struct{})
			for i := range values {
				if len(values[i]) > 0 {
					set[values[i][0]] = struct{}{}
				}
			}
			out[attrType] = set
		}
	}
	return out
}

func extractExtraMax(raw json.RawMessage) map[uint32][2]uint32 {
	out := make(map[uint32][2]uint32)
	if len(raw) == 0 {
		return out
	}
	var list [][]uint32
	if err := decodeUsageArg(raw, &list); err == nil {
		for i := range list {
			if len(list[i]) < 2 {
				continue
			}
			out[uint32(i+1)] = [2]uint32{list[i][0], list[i][1]}
		}
		return out
	}

	var obj map[string][]uint32
	if err := decodeUsageArg(raw, &obj); err == nil {
		for key, values := range obj {
			if len(values) < 2 {
				continue
			}
			attrType := parseUint32Key(key)
			if attrType == 0 {
				continue
			}
			out[attrType] = [2]uint32{values[0], values[1]}
		}
	}
	return out
}

func skillMaterialAtLevel(cfg *islandCharaSkillConfig, level uint32) ([][2]uint32, bool) {
	if level == 0 {
		return nil, false
	}
	var list [][][]uint32
	if err := decodeUsageArg(cfg.Material, &list); err != nil {
		return nil, false
	}
	idx := int(level - 1)
	if idx < 0 || idx >= len(list) {
		return nil, false
	}
	materials := make([][2]uint32, 0, len(list[idx]))
	for i := range list[idx] {
		if len(list[idx][i]) < 2 || list[idx][i][0] == 0 || list[idx][i][1] == 0 {
			continue
		}
		materials = append(materials, [2]uint32{list[idx][i][0], list[idx][i][1]})
	}
	if len(materials) == 0 {
		return nil, false
	}
	return materials, true
}

func skillMaxLevel(cfg *islandCharaSkillConfig) uint32 {
	var list []json.RawMessage
	if err := decodeUsageArg(cfg.SkillEffect, &list); err == nil {
		if len(list) > 0 {
			return uint32(len(list))
		}
	}
	var list2 []any
	if err := decodeUsageArg(cfg.SkillEffect, &list2); err == nil {
		return uint32(len(list2))
	}
	return 1
}

type islandGiftEffect struct {
	Energy  uint32
	BuffIDs []uint32
}

func parseGiftEffects(raw json.RawMessage, favorite bool) (islandGiftEffect, bool) {
	branches, ok := parseGiftEffectBranches(raw)
	if !ok {
		return islandGiftEffect{}, false
	}
	if favorite {
		if len(branches) >= 2 {
			return branches[1], true
		}
		return branches[0], true
	}
	return branches[0], true
}

func parseGiftEffectBranches(raw json.RawMessage) ([]islandGiftEffect, bool) {
	normalized, err := normalizeUsageArg(raw)
	if err != nil || len(normalized) == 0 {
		return nil, false
	}
	var root []any
	if err := json.Unmarshal(normalized, &root); err != nil {
		return nil, false
	}
	branches := make([]islandGiftEffect, 0)
	for _, entry := range root {
		branch, ok := parseGiftEffectBranch(entry)
		if !ok {
			continue
		}
		branches = append(branches, branch)
	}
	if len(branches) == 0 {
		return nil, false
	}
	return branches, true
}

func parseGiftEffectBranch(raw any) (islandGiftEffect, bool) {
	array, ok := raw.([]any)
	if !ok || len(array) == 0 {
		return islandGiftEffect{}, false
	}
	energy := parseGiftUint32Any(array[0])
	buffs := []uint32{}
	for i := 1; i < len(array); i++ {
		if list, ok := array[i].([]any); ok {
			for j := range list {
				if value := parseGiftUint32Any(list[j]); value > 0 {
					buffs = append(buffs, value)
				}
			}
			continue
		}
		if value := parseGiftUint32Any(array[i]); value > 0 {
			buffs = append(buffs, value)
		}
	}
	return islandGiftEffect{Energy: energy, BuffIDs: buffs}, true
}

func parseGiftUint32Any(v any) uint32 {
	switch value := v.(type) {
	case float64:
		if value < 0 {
			return 0
		}
		return uint32(value)
	case int:
		if value < 0 {
			return 0
		}
		return uint32(value)
	case uint32:
		return value
	case string:
		parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
		return uint32(parsed)
	default:
		return 0
	}
}

func shipHasFavoriteGift(template *islandCharaTemplateConfig, giftID uint32) bool {
	if len(template.Favorite) == 0 {
		return false
	}
	var list []uint32
	if err := decodeUsageArg(template.Favorite, &list); err == nil {
		for i := range list {
			if list[i] == giftID {
				return true
			}
		}
		return false
	}
	var wrapped [][]uint32
	if err := decodeUsageArg(template.Favorite, &wrapped); err == nil {
		for i := range wrapped {
			for j := range wrapped[i] {
				if wrapped[i][j] == giftID {
					return true
				}
			}
		}
	}
	return false
}

func upsertShipBuffsWithConflict(current []orm.IslandShipBuff, additions []uint32, now uint32) []orm.IslandShipBuff {
	result := append([]orm.IslandShipBuff(nil), current...)
	for _, buffID := range additions {
		if buffID == 0 {
			continue
		}
		cfg, found, err := loadIslandBuffTemplateConfig(buffID)
		if err != nil || !found {
			result = append(result, orm.IslandShipBuff{ID: buffID, StartTime: now})
			continue
		}
		if cfg.DuelType != 0 && cfg.DuelID != 0 {
			filtered := result[:0]
			for i := range result {
				other, foundOther, _ := loadIslandBuffTemplateConfig(result[i].ID)
				if foundOther && other.DuelType == cfg.DuelType && other.DuelID == cfg.DuelID {
					continue
				}
				filtered = append(filtered, result[i])
			}
			result = filtered
		}
		result = append(result, orm.IslandShipBuff{ID: buffID, StartTime: now})
	}
	return result
}

func loadIslandDropList(dropID uint32) ([]*protobuf.DROPINFO, error) {
	if dropID == 0 {
		return []*protobuf.DROPINFO{}, nil
	}
	cfg, found, err := loadIslandDropDataConfig(dropID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, db.ErrNotFound
	}

	for _, raw := range []json.RawMessage{cfg.DropList, cfg.Display, cfg.Award} {
		drops := parseDropListRaw(raw)
		if len(drops) > 0 {
			return drops, nil
		}
	}
	return []*protobuf.DROPINFO{}, nil
}

func parseDropListRaw(raw json.RawMessage) []*protobuf.DROPINFO {
	if len(raw) == 0 {
		return nil
	}
	var list [][]uint32
	if err := decodeUsageArg(raw, &list); err != nil {
		return nil
	}
	drops := make([]*protobuf.DROPINFO, 0, len(list))
	for i := range list {
		if len(list[i]) < 3 {
			continue
		}
		drops = append(drops, newDropInfo(list[i][0], list[i][1], list[i][2]))
	}
	return drops
}

func currentDayStartUnix(now time.Time) uint32 {
	y, m, d := now.UTC().Date()
	return uint32(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix())
}

func parseIslandSetUintValue(key string, fallback uint32) uint32 {
	return loadIslandSetInt(key, fallback)
}
