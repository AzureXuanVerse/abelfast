package island

import (
	"encoding/json"
	"strconv"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
)

const (
	islandCollectFragmentCategory   = "ShareCfg/island_collect_fragment.json"
	islandCollectFragmentCategoryLC = "sharecfgdata/island_collect_fragment.json"
	islandCollectionCategory        = "ShareCfg/island_collection.json"
	islandCollectionCategoryLC      = "sharecfgdata/island_collection.json"
)

type islandWildGatherCollectTemplate struct {
	ID          uint32     `json:"id"`
	Show        uint32     `json:"show"`
	DropDisplay [][]uint32 `json:"drop_display"`
	DropList    [][]uint32 `json:"drop_list"`
	Award       [][]uint32 `json:"award_display"`
}

type islandCollectFragmentTemplate struct {
	ID           uint32 `json:"id"`
	CollectionID uint32 `json:"collection_id"`
	Show         uint32 `json:"show"`
}

type islandCollectionTemplate struct {
	ID           uint32   `json:"id"`
	FragmentList []uint32 `json:"fragment_list"`
}

func loadIslandWildGatherCollectTemplate(gatherID uint32) (*islandWildGatherCollectTemplate, error) {
	entry, err := orm.GetConfigEntry(islandWildGatherCategory, strconv.FormatUint(uint64(gatherID), 10))
	if err != nil {
		if db.IsNotFound(err) {
			entry, err = orm.GetConfigEntry(islandWildGatherCategoryLC, strconv.FormatUint(uint64(gatherID), 10))
		}
		if err != nil {
			return nil, err
		}
	}
	var template islandWildGatherCollectTemplate
	if err := json.Unmarshal(entry.Data, &template); err != nil {
		return nil, err
	}
	if template.ID == 0 {
		template.ID = gatherID
	}
	return &template, nil
}

func loadIslandCollectFragmentTemplate(fragmentID uint32) (*islandCollectFragmentTemplate, error) {
	entry, err := orm.GetConfigEntry(islandCollectFragmentCategory, strconv.FormatUint(uint64(fragmentID), 10))
	if err != nil {
		if db.IsNotFound(err) {
			entry, err = orm.GetConfigEntry(islandCollectFragmentCategoryLC, strconv.FormatUint(uint64(fragmentID), 10))
		}
		if err != nil {
			return nil, err
		}
	}
	var template islandCollectFragmentTemplate
	if err := json.Unmarshal(entry.Data, &template); err != nil {
		return nil, err
	}
	if template.ID == 0 {
		template.ID = fragmentID
	}
	return &template, nil
}

func loadIslandCollectionTemplate(collectID uint32) (*islandCollectionTemplate, error) {
	entry, err := orm.GetConfigEntry(islandCollectionCategory, strconv.FormatUint(uint64(collectID), 10))
	if err != nil {
		if db.IsNotFound(err) {
			entry, err = orm.GetConfigEntry(islandCollectionCategoryLC, strconv.FormatUint(uint64(collectID), 10))
		}
		if err != nil {
			return nil, err
		}
	}
	var template islandCollectionTemplate
	if err := json.Unmarshal(entry.Data, &template); err != nil {
		return nil, err
	}
	if template.ID == 0 {
		template.ID = collectID
	}
	return &template, nil
}
