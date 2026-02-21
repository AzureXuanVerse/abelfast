package island

import (
	"encoding/json"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	islandCardDIYCategory            = "ShareCfg/island_card_diy.json"
	islandCardDIYCategoryLC          = "sharecfgdata/island_card_diy.json"
	islandCardLabelCategory          = "ShareCfg/island_card_label.json"
	islandCardLabelCategoryLC        = "sharecfgdata/island_card_label.json"
	islandIllustratedGuideCategory   = "ShareCfg/island_illustrated_guide.json"
	islandIllustratedGuideCategoryLC = "sharecfgdata/island_illustrated_guide.json"
	islandCollectionRewardCategory   = "ShareCfg/island_collection_reward.json"
	islandCollectionRewardCategoryLC = "sharecfgdata/island_collection_reward.json"

	islandCardPhotoTypeID = 1

	islandCardFlagSocial = 1
	islandCardFlagLabel  = 2
)

type islandCardDIYConfig struct {
	ID uint32 `json:"id"`
}

type islandCardLabelConfig struct {
	ID uint32 `json:"id"`
}

type islandAchievementCardConfig struct {
	ID    uint32 `json:"id"`
	Group uint32 `json:"group"`
	Stage uint32 `json:"stage"`
}

type islandIllustratedGuideConfig struct {
	ID             uint32     `json:"id"`
	Type           uint32     `json:"type"`
	CollectAdd     uint32     `json:"collect_add"`
	CollectUpgrade [][]uint32 `json:"collect_upgrade"`
	CollectStar    [][]uint32 `json:"collect_star"`
	AwardUnlock    [][]uint32 `json:"award_unlock"`
}

type islandCollectionRewardConfig struct {
	ID           uint32          `json:"id"`
	Type         uint32          `json:"type"`
	NeedExp      uint32          `json:"need_exp"`
	AwardDisplay json.RawMessage `json:"award_display"`
}

func loadIslandCardDIYIDs() (map[uint32]struct{}, error) {
	entries, err := listConfigEntriesWithFallback(islandCardDIYCategory, islandCardDIYCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}
	result := make(map[uint32]struct{})
	for _, entry := range entries {
		rows := []islandCardDIYConfig{}
		if err := json.Unmarshal(entry.Data, &rows); err == nil {
			for _, row := range rows {
				if row.ID > 0 {
					result[row.ID] = struct{}{}
				}
			}
			continue
		}

		row := islandCardDIYConfig{}
		if err := json.Unmarshal(entry.Data, &row); err == nil {
			if row.ID == 0 {
				row.ID = parseUint32Key(entry.Key)
			}
			if row.ID > 0 {
				result[row.ID] = struct{}{}
			}
		}
	}
	return result, nil
}

func loadIslandCardLabelIDs() (map[uint32]struct{}, error) {
	entries, err := listConfigEntriesWithFallback(islandCardLabelCategory, islandCardLabelCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}
	result := make(map[uint32]struct{})
	for _, entry := range entries {
		rows := []islandCardLabelConfig{}
		if err := json.Unmarshal(entry.Data, &rows); err == nil {
			for _, row := range rows {
				if row.ID > 0 {
					result[row.ID] = struct{}{}
				}
			}
			continue
		}
		row := islandCardLabelConfig{}
		if err := json.Unmarshal(entry.Data, &row); err == nil {
			if row.ID == 0 {
				row.ID = parseUint32Key(entry.Key)
			}
			if row.ID > 0 {
				result[row.ID] = struct{}{}
			}
		}
	}
	return result, nil
}

func loadIslandAchievementCardConfig() (map[uint32]islandAchievementCardConfig, map[uint32][]uint32, error) {
	entries, err := listConfigEntriesWithFallback(islandAchievementCategory, islandAchievementCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[uint32]islandAchievementCardConfig)
	byGroup := make(map[uint32][]uint32)
	for _, entry := range entries {
		row := islandAchievementCardConfig{}
		if err := json.Unmarshal(entry.Data, &row); err != nil {
			continue
		}
		if row.ID == 0 {
			row.ID = parseUint32Key(entry.Key)
		}
		if row.ID == 0 || row.Group == 0 {
			continue
		}
		byID[row.ID] = row
		byGroup[row.Group] = append(byGroup[row.Group], row.ID)
	}
	for group := range byGroup {
		sort.Slice(byGroup[group], func(i, j int) bool {
			left := byID[byGroup[group][i]]
			right := byID[byGroup[group][j]]
			if left.Stage == right.Stage {
				return left.ID < right.ID
			}
			return left.Stage < right.Stage
		})
	}
	return byID, byGroup, nil
}

func loadIslandIllustratedGuideConfig() (map[uint32]islandIllustratedGuideConfig, error) {
	entries, err := listConfigEntriesWithFallback(islandIllustratedGuideCategory, islandIllustratedGuideCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, err
	}
	result := make(map[uint32]islandIllustratedGuideConfig)
	for _, entry := range entries {
		rows := []islandIllustratedGuideConfig{}
		if err := json.Unmarshal(entry.Data, &rows); err == nil {
			for _, row := range rows {
				if row.ID > 0 {
					result[row.ID] = row
				}
			}
			continue
		}

		row := islandIllustratedGuideConfig{}
		if err := json.Unmarshal(entry.Data, &row); err == nil {
			if row.ID == 0 {
				row.ID = parseUint32Key(entry.Key)
			}
			if row.ID > 0 {
				result[row.ID] = row
			}
		}
	}
	return result, nil
}

func loadIslandCollectionRewardConfig() (map[uint32]islandCollectionRewardConfig, map[uint32][]uint32, error) {
	entries, err := listConfigEntriesWithFallback(islandCollectionRewardCategory, islandCollectionRewardCategoryLC, orm.ListConfigEntries)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[uint32]islandCollectionRewardConfig)
	byType := make(map[uint32][]uint32)
	for _, entry := range entries {
		rows := []islandCollectionRewardConfig{}
		if err := json.Unmarshal(entry.Data, &rows); err == nil {
			for _, row := range rows {
				if row.ID == 0 {
					continue
				}
				byID[row.ID] = row
				byType[row.Type] = append(byType[row.Type], row.ID)
			}
			continue
		}

		row := islandCollectionRewardConfig{}
		if err := json.Unmarshal(entry.Data, &row); err == nil {
			if row.ID == 0 {
				row.ID = parseUint32Key(entry.Key)
			}
			if row.ID == 0 {
				continue
			}
			byID[row.ID] = row
			byType[row.Type] = append(byType[row.Type], row.ID)
		}
	}

	for rewardType := range byType {
		sort.Slice(byType[rewardType], func(i, j int) bool {
			left := byID[byType[rewardType][i]]
			right := byID[byType[rewardType][j]]
			if left.NeedExp == right.NeedExp {
				return left.ID < right.ID
			}
			return left.NeedExp < right.NeedExp
		})
	}

	return byID, byType, nil
}

func parseIslandRewardDisplay(raw json.RawMessage) ([][]uint32, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var matrix [][]uint32
	if err := json.Unmarshal(raw, &matrix); err == nil {
		return matrix, nil
	}
	var single []uint32
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, err
	}
	if len(single) >= 3 {
		return [][]uint32{single[:3]}, nil
	}
	return nil, nil
}

func buildIslandLabelList(labels []orm.IslandCardLabelCount) []*protobuf.PB_ISLAND_LABEL {
	if len(labels) == 0 {
		return []*protobuf.PB_ISLAND_LABEL{}
	}
	sort.Slice(labels, func(i, j int) bool { return labels[i].ID < labels[j].ID })
	out := make([]*protobuf.PB_ISLAND_LABEL, 0, len(labels))
	for _, label := range labels {
		out = append(out, &protobuf.PB_ISLAND_LABEL{Id: proto.Uint32(label.ID), Num: proto.Uint32(label.Num)})
	}
	return out
}

func buildIslandBookCollectProto(values []orm.IslandBookCollectEntry) []*protobuf.PB_BOOK_COLLECT {
	out := make([]*protobuf.PB_BOOK_COLLECT, 0, len(values))
	for _, value := range values {
		lvList := make([]*protobuf.PB_LV_COLLECT, 0, len(value.LvList))
		for _, level := range value.LvList {
			lvList = append(lvList, &protobuf.PB_LV_COLLECT{Lv: proto.Uint32(level.Lv), Value: proto.Uint32(level.Value)})
		}
		starList := make([]*protobuf.PB_LV_COLLECT, 0, len(value.StarList))
		for _, level := range value.StarList {
			starList = append(starList, &protobuf.PB_LV_COLLECT{Lv: proto.Uint32(level.Lv), Value: proto.Uint32(level.Value)})
		}
		out = append(out, &protobuf.PB_BOOK_COLLECT{Id: proto.Uint32(value.ID), Base: proto.Uint32(value.Base), LvList: lvList, StarList: starList})
	}
	return out
}

func normalizeCardWord(word string) string {
	return normalizeIslandName(word)
}

func isCardWordValid(word string) bool {
	runeCount := utf8.RuneCountInString(word)
	return runeCount >= 4 && runeCount <= 60
}

func parsePictureID(picture string) uint32 {
	parsed, err := strconv.ParseUint(picture, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(parsed)
}
