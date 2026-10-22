package admin

import (
	"sort"
	"strings"
	"sync"

	"github.com/ggmolly/belfast/internal/gamedata"
)

// activityKinds names the activity_template types (ActivityConst
// ACTIVITY_TYPE_*, model/const/activityconst.lua).
var activityKinds = map[uint32]string{
	1:    "Build event",
	2:    "Parameter",
	3:    "7-day login",
	4:    "Level rewards",
	5:    "Levelplan",
	6:    "Monthly sign-in",
	7:    "Military exercise",
	8:    "Login rewards",
	9:    "Chargeaward",
	10:   "Grafting",
	11:   "Event",
	12:   "Main event",
	13:   "Tasks",
	14:   "Shop",
	15:   "Npc collection",
	16:   "Event points",
	18:   "Task list",
	19:   "Vote",
	20:   "Pachinko",
	21:   "Puzzle",
	22:   "Lottery",
	24:   "Challenge rank",
	26:   "Build",
	27:   "Shop street",
	29:   "Love letter",
	30:   "Buff",
	32:   "Monster hunt",
	33:   "Card pairs",
	36:   "Extra chapter rank",
	37:   "Challenge",
	38:   "Summary",
	39:   "Task list monitor",
	40:   "Task res",
	41:   "Event bag",
	42:   "Returning commander",
	43:   "Coloring",
	44:   "Guide tasks",
	45:   "Link link",
	46:   "Boss rank",
	48:   "Shop discount",
	49:   "Lottery award",
	50:   "Dodgem",
	51:   "Monopoly",
	52:   "Boss battle mark 2",
	53:   "Blackwhite",
	54:   "Cruise pass",
	55:   "Buildship pray",
	56:   "Return award",
	57:   "Mini-game",
	58:   "Ship buff",
	59:   "Story award",
	60:   "Juustagram",
	61:   "Turntable",
	63:   "Shrine",
	64:   "Red packets",
	66:   "Pizza pt",
	67:   "Building buff",
	68:   "Skin coupon",
	69:   "Blueprint catchup",
	70:   "Client display",
	71:   "Fresh tec catchup",
	72:   "Submarine run",
	73:   "Expedition",
	74:   "Pt buff",
	75:   "Airfight battle",
	76:   "Collection event",
	77:   "Shop progress reward",
	78:   "Shake beads",
	79:   "Worldinpicture",
	80:   "Build free",
	81:   "April reward",
	82:   "New commander tasks",
	83:   "New commander shop",
	84:   "New commander gifts",
	85:   "New commander build",
	86:   "Task pool",
	87:   "Event points",
	88:   "Atelier",
	89:   "Atelier tasks",
	90:   "Red packet lotter",
	91:   "Hotspring",
	92:   "Firework",
	93:   "Boss rush",
	94:   "Extra bossrush rank",
	95:   "Island",
	96:   "Workbench",
	97:   "Building buff 2",
	98:   "Flower field",
	99:   "Sculpture",
	100:  "Hotspring 2",
	101:  "Survey",
	102:  "Card puzzle",
	103:  "Link collect",
	104:  "Feast",
	105:  "Zuma",
	106:  "Castle",
	107:  "Train",
	108:  "Gift up",
	109:  "Sign-in",
	110:  "Random daily task",
	111:  "Bosssingle",
	112:  "Event single",
	113:  "Store date",
	114:  "Liner",
	115:  "Ur exchange",
	116:  "Town",
	118:  "Dreamland",
	119:  "Shop selectable",
	120:  "Black Friday shop",
	121:  "Not traceable",
	123:  "Bosssingle variable",
	124:  "Skin coupon counting",
	125:  "Holiday villa",
	126:  "Daily stage bonus",
	127:  "Stronghold",
	128:  "Common buff",
	129:  "City rebuild",
	130:  "Pt hei5",
	131:  "Skin fake package",
	132:  "Boss rush dal collab",
	133:  "Love letter up",
	134:  "Love letter mail",
	135:  "Town2",
	136:  "Auction",
	137:  "Reverse pacman",
	140:  "Mall",
	141:  "Login record",
	142:  "Times fake package",
	174:  "Pt buff mark2",
	800:  "Island draw award",
	801:  "Island bar",
	890:  "Island game pt",
	999:  "Island cheate tavern",
	1001: "Puzzle connect",
}

// lifeTables are the config names the Activities, Shops and Archives screens
// show, read once.
type lifeTables struct {
	// activity names: the mini-events' (activity_task_permanent) and the
	// event maps' (expedition_data_by_map on_activity)
	activities    map[uint32]string
	medals        map[uint32]named // medal_template
	storeups      map[uint32]storeupRow
	handbookTasks map[uint32]string // tutorial_handbook_task id: its book and phase
	specials      map[uint32]string // secretary_special_ship
	iconFrames    map[uint32]string // item_data_frame (playerTables)
	chatFrames    map[uint32]string // item_data_chat (playerTables)
	covers        map[uint32]string // livingarea_cover
	coverIcons    map[uint32]string // livingarea_cover icon_res: livingareacover/<icon_res>
	children      map[uint32]string // child2_data, the new Project Identity
	// player_resource itemid: the virtual item a resource is named after
	resourceItems map[uint32]uint32
}

type storeupRow struct {
	Name         string     `json:"name"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

var loadLifeTables = sync.OnceValues(func() (*lifeTables, error) {
	tables := &lifeTables{activities: map[uint32]string{}, handbookTasks: map[uint32]string{}}
	type nameRow struct {
		Name string `json:"name"`
	}
	permanent, err := gamedata.Table[struct {
		Name string `json:"activity_name"`
	}]("activity_task_permanent")
	if err != nil {
		return nil, err
	}
	maps, err := gamedata.Table[struct {
		Name     string `json:"name"`
		Activity uint32 `json:"on_activity"`
	}]("expedition_data_by_map")
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(maps))
	for id := range maps {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		row := maps[id]
		name, _, _ := strings.Cut(row.Name, "||")
		if name = strings.TrimSpace(name); row.Activity != 0 && name != "" && tables.activities[row.Activity] == "" {
			tables.activities[row.Activity] = name
		}
	}
	for id, row := range permanent {
		if name := strings.TrimSpace(row.Name); name != "" {
			tables.activities[id] = name
		}
	}
	medals, err := gamedata.Table[tableRow]("medal_template")
	if err != nil {
		return nil, err
	}
	tables.medals = make(map[uint32]named, len(medals))
	for id, row := range medals {
		tables.medals[id] = named{Name: row.Name}
	}
	if tables.storeups, err = gamedata.Table[storeupRow]("storeup_data_template"); err != nil {
		return nil, err
	}
	books, err := gamedata.Table[struct {
		Name string   `json:"name"`
		Tags []uint32 `json:"tag_list"`
	}]("tutorial_handbook")
	if err != nil {
		return nil, err
	}
	phases, err := gamedata.Table[nameRow]("tutorial_handbook_task")
	if err != nil {
		return nil, err
	}
	for id, phase := range phases {
		tables.handbookTasks[id] = phase.Name
	}
	for _, book := range books {
		for _, tag := range book.Tags {
			if phase, ok := phases[tag]; ok {
				tables.handbookTasks[tag] = strings.Join(strings.Fields(book.Name), " ") + " · " + phase.Name
			}
		}
	}
	names := func(table string) (map[uint32]string, error) {
		rows, err := gamedata.Table[nameRow](table)
		if err != nil {
			return nil, err
		}
		byID := make(map[uint32]string, len(rows))
		for id, row := range rows {
			byID[id] = row.Name
		}
		return byID, nil
	}
	players, err := loadPlayerTables()
	if err != nil {
		return nil, err
	}
	tables.iconFrames, tables.chatFrames = nameMap(players.names[dropIconFrame]), nameMap(players.names[dropChatFrame])
	for table, into := range map[string]*map[uint32]string{
		"secretary_special_ship": &tables.specials,
		"livingarea_cover":       &tables.covers,
		"child2_data":            &tables.children,
	} {
		if *into, err = names(table); err != nil {
			return nil, err
		}
	}
	covers, err := gamedata.Table[struct {
		IconRes string `json:"icon_res"`
	}]("livingarea_cover")
	if err != nil {
		return nil, err
	}
	tables.coverIcons = make(map[uint32]string, len(covers))
	for id, row := range covers {
		if row.IconRes != "" {
			tables.coverIcons[id] = "livingareacover/" + strings.ToLower(row.IconRes)
		}
	}
	resources, err := gamedata.Table[struct {
		ItemID uint32 `json:"itemid"`
	}]("player_resource")
	if err != nil {
		return nil, err
	}
	tables.resourceItems = make(map[uint32]uint32, len(resources))
	for id, row := range resources {
		tables.resourceItems[id] = row.ItemID
	}
	return tables, nil
})

// nameMap is the names of rows.
func nameMap(rows map[uint32]named) map[uint32]string {
	byID := make(map[uint32]string, len(rows))
	for id, row := range rows {
		byID[id] = row.Name
	}
	return byID
}

// shopEffects name the shop_template extensions (effect_args of the type 0
// goods, ExtendCommand).
var shopEffects = map[string]string{
	"ship_bag_size":      "Dock space",
	"equip_bag_size":     "Gear storage",
	"commander_bag_size": "Meowfficer storage",
	"dorm_exp_pos":       "Backyard training slot",
	"dorm_food_max":      "Backyard food capacity",
	"dorm_floor":         "Backyard floor",
	"skill_room_pos":     "Tactical Class slot",
	"tradingport_level":  "Merchant upgrade",
	"oilfield_level":     "Oil field upgrade",
	"class_room_level":   "Class room upgrade",
}
