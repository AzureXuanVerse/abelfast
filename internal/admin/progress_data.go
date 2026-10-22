package admin

import (
	"encoding/json"
	"sort"
	"sync"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// The config tables the timers, progress and research screens read, loaded
// on first use.

type progressTables struct {
	pools       map[uint32]struct{ Name string } // ship_data_create_material
	collections map[uint32]struct {
		Title       string `json:"title"`
		CollectTime int64  `json:"collect_time"`
	}
	skills   map[uint32]struct{ Name string } // skill_data_template
	chapters map[uint32]progChapterRow
	maps     map[uint32]progMapRow
	tasks    map[uint32]struct {
		Name      string `json:"name"`
		Desc      string `json:"desc"`
		Type      uint32 `json:"type"`
		TargetNum uint32 `json:"target_num"`
	}
	weekly map[uint32]struct {
		Desc      string `json:"desc"`
		TargetNum uint32 `json:"target_num"`
	}
	projects map[uint32]struct {
		Name     string `json:"name"`
		SubName  string `json:"sub_name"`
		Time     int64  `json:"time"`
		Version  uint32 `json:"blueprint_version"`
		LvLimit  uint32 `json:"lv_limit"`
		Rarity   uint32 `json:"rarity"`
		Category uint32 `json:"type"`
	}
	blueprints map[uint32]struct {
		Version uint32 `json:"blueprint_version"`
	}
	catchups map[uint32]struct {
		CharChoice []uint32 `json:"char_choice"`
		URChar     []uint32 `json:"ur_char"`
		ObtainMax  uint32   `json:"obtain_max"`
		PerUR      uint32   `json:"obtain_max_per_ur"`
	}
	techGroups map[uint32]struct {
		Name   string   `json:"name"`
		Nation []uint32 `json:"nation"`
	}
	shipTypes map[uint32]struct {
		TypeName string `json:"type_name"`
	}
	shipGroups int

	// the campaign: the normal maps in order, the stages of each map
	normalMaps []uint32
	stages     map[uint32][]uint32
}

// progChapterRow is what the campaign shows of a chapter_template row: its
// stars are star_require_N (ChapterConst.AchieveType) with num_N.
type progChapterRow struct {
	Map          uint32 `json:"map"`
	ChapterName  string `json:"chapter_name"`
	Name         string `json:"name"`
	StarRequire1 uint32 `json:"star_require_1"`
	StarRequire2 uint32 `json:"star_require_2"`
	StarRequire3 uint32 `json:"star_require_3"`
	Num1         uint32 `json:"num_1"`
	Num2         uint32 `json:"num_2"`
	Num3         uint32 `json:"num_3"`
	AmmoTotal    uint32 `json:"ammo_total"`
	UnlockLevel  uint32 `json:"unlocklevel"`
}

// progMapRow is an expedition_data_by_map row: type 1 the normal campaign, 2
// its hard mode (bind_map the other mode's map).
type progMapRow struct {
	Name       string `json:"name"`
	Title      string `json:"title"`
	Type       uint32 `json:"type"`
	BindMap    uint32 `json:"bind_map"`
	LevelLimit uint32 `json:"level_limit"`
}

const (
	progMapNormal = 1
	progMapHard   = 2
)

var loadProgressTables = sync.OnceValues(func() (*progressTables, error) {
	t := &progressTables{}
	var groups map[uint32]json.RawMessage
	err := gamedata.LoadAll(
		gamedata.Load("ship_data_create_material", &t.pools),
		gamedata.Load("collection_template", &t.collections),
		gamedata.Load("skill_data_template", &t.skills),
		gamedata.Load("chapter_template", &t.chapters),
		gamedata.Load("expedition_data_by_map", &t.maps),
		gamedata.Load("task_data_template", &t.tasks),
		gamedata.Load("weekly_task_template", &t.weekly),
		gamedata.Load("technology_data_template", &t.projects),
		gamedata.Load("ship_data_blueprint", &t.blueprints),
		gamedata.Load("technology_catchup_template", &t.catchups),
		gamedata.Load("fleet_tech_group", &t.techGroups),
		gamedata.Load("ship_data_by_type", &t.shipTypes),
		gamedata.Load("ship_data_group", &groups),
	)
	if err != nil {
		return nil, err
	}
	t.shipGroups = len(groups)
	t.stages = map[uint32][]uint32{}
	for id, row := range t.chapters {
		if kind := t.maps[row.Map].Type; kind == progMapNormal || kind == progMapHard {
			t.stages[row.Map] = append(t.stages[row.Map], id)
		}
	}
	for mapID, stages := range t.stages {
		sort.Slice(stages, func(i, j int) bool { return stages[i] < stages[j] })
		if t.maps[mapID].Type == progMapNormal {
			t.normalMaps = append(t.normalMaps, mapID)
		}
	}
	sort.Slice(t.normalMaps, func(i, j int) bool { return t.normalMaps[i] < t.normalMaps[j] })
	return t, nil
})

// stageName is a campaign stage's name ("7–3"), "" for none.
func (t *progressTables) stageName(id uint32) string { return t.chapters[id].ChapterName }
