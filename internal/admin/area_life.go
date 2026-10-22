package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
)

// registerLife registers the API of a commander's combat, backyard, friends, shops, activities and archives.
func (server *Server) registerLife(api apiFunc) {
	api("GET /api/commanders/{id}/shops", server.handleShops)
	api("POST /api/commanders/{id}/shops/reset", server.handleShopReset)
	api("POST /api/commanders/{id}/akashi", server.handleAkashi)
	api("POST /api/commanders/{id}/requisitions/reset", server.handleRequisitionsReset)
	api("GET /api/commanders/{id}/activities", server.handleActivities)
	api("POST /api/commanders/{id}/activities/reset", server.handleActivityClear(true))
	api("POST /api/commanders/{id}/activities/delete", server.handleActivityClear(false))
	api("GET /api/commanders/{id}/archives", server.handleArchives)
	api("POST /api/commanders/{id}/archives/medals/{medal}/reset", server.handleMedalReset)
	api("POST /api/commanders/{id}/archives/seen/{what}/reset", server.handleSeenReset)
	api("POST /api/commanders/{id}/archives/look", server.handleLook)
}

// --- shops

type lifeThing struct {
	Type   uint32 `json:"type"`
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Rarity uint32 `json:"rarity"`
	Icon   string `json:"icon,omitempty"`
	Count  uint32 `json:"count"`
}

type lifeGoods struct {
	ID     uint32    `json:"id"`
	Group  bool      `json:"group,omitempty"`
	Goods  lifeThing `json:"goods"`
	Price  lifeThing `json:"price"`
	Bought uint32    `json:"bought"`
	Limit  uint32    `json:"limit"`
	Never  bool      `json:"never,omitempty"`
}

type lifeShop struct {
	Kind      string      `json:"kind"`
	Name      string      `json:"name"`
	Activity  uint32      `json:"activity,omitempty"`
	ResetAt   int64       `json:"reset_at"`
	Refreshes uint32      `json:"refreshes"`
	InGuild   bool        `json:"in_guild"`
	Bought    uint32      `json:"bought"`
	Goods     []lifeGoods `json:"goods"`
}

var shopNames = map[string]string{
	game.AdminShopStreet:    "General shop",
	game.AdminShopArena:     "Merit shop",
	game.AdminShopOil:       "Oil",
	game.AdminShopPacks:     "Supply packs",
	game.AdminShopOther:     "Akashi's shop",
	game.AdminShopMedal:     "Medal shop",
	game.AdminShopCore:      "Core Data exchange",
	game.AdminShopPrototype: "Prototype shop",
	game.AdminShopGuild:     "Guild shop",
	game.AdminShopPrize:     "Prize Shop",
	game.AdminShopNewServer: "New commander shop",
}

// thing names a drop, or a shop_template extension (effect).
func (server *Server) thing(kind, id, count uint32, effect string) lifeThing {
	if kind == 0 && effect != "" {
		name := shopEffects[effect]
		if name == "" {
			name = humanize(effect)
		}
		return lifeThing{Name: name, Count: count}
	}
	row := server.names[kind][id]
	name := strings.TrimSpace(row.Name)
	if tables, err := loadLifeTables(); err == nil && kind == dropResource {
		if virtual := strings.TrimSpace(server.names[dropVItem][tables.resourceItems[id]].Name); virtual != "" {
			name = virtual
		}
	}
	if name == "" {
		name = fmt.Sprintf("#%d", id)
	}
	return lifeThing{Type: kind, ID: id, Name: name, Rarity: row.Rarity, Icon: row.Icon, Count: count}
}

// humanize turns a config key (dorm_food_max, Data1List) into words.
func humanize(key string) string {
	var out []rune
	for i, r := range key {
		switch {
		case r == '_':
			out = append(out, ' ')
		case unicode.IsUpper(r) && i > 0:
			out = append(out, ' ', unicode.ToLower(r))
		default:
			out = append(out, r)
		}
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return text
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

type lifeShops struct {
	Shops  []lifeShop `json:"shops"`
	Akashi struct {
		Exp      uint32 `json:"exp"`
		Taps     uint32 `json:"taps"`
		Daily    uint32 `json:"daily"`
		TappedAt int64  `json:"tapped_at"`
	} `json:"akashi"`
	Requisitions struct {
		Count   uint32 `json:"count"`
		Limit   uint32 `json:"limit"`
		ResetAt int64  `json:"reset_at"`
	} `json:"requisitions"`
}

// handleShops is what the commander bought in each shop, Akashi's affection
// and this month's requisitions.
func (server *Server) handleShops(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	shops, err := server.Game.AdminShops(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	reply := lifeShops{Shops: []lifeShop{}}
	for _, shop := range shops {
		row := lifeShop{Kind: shop.Kind, Name: shopNames[shop.Kind], Activity: shop.Activity, ResetAt: shop.ResetAt, Refreshes: shop.Refreshes, InGuild: shop.InGuild, Goods: []lifeGoods{}}
		if shop.ActType == 120 { // ACTIVITY_TYPE_BLACK_FRIDAY_SHOP
			row.Name = "Black Friday shop"
		}
		for _, goods := range shop.Goods {
			row.Bought += goods.Bought
			row.Goods = append(row.Goods, lifeGoods{
				ID: goods.ID, Group: goods.Group, Bought: goods.Bought, Limit: goods.Limit, Never: goods.Never,
				Goods: server.thing(goods.Type, goods.Item, goods.Num, goods.Effect),
				Price: server.thing(goods.PriceType, goods.PriceID, goods.Price, ""),
			})
		}
		reply.Shops = append(reply.Shops, row)
	}
	akashi, daily, err := server.Game.AdminAkashi(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	reply.Akashi.Exp, reply.Akashi.Taps, reply.Akashi.Daily, reply.Akashi.TappedAt = akashi.Exp, akashi.Taps, daily, akashi.TappedAt
	count, limit, resetAt, err := server.Game.AdminRequisitions(ctx, commander.ID)
	if err != nil {
		return nil, err
	}
	reply.Requisitions.Count, reply.Requisitions.Limit, reply.Requisitions.ResetAt = count, limit, resetAt
	return reply, nil
}

// handleShopReset forgets purchases of a shop: one goods (id, group for a
// shop_template group) or all of them (id 0), or the guild shop's
// refreshes.
func (server *Server) handleShopReset(r *http.Request) (any, error) {
	type body struct {
		Kind      string `json:"kind"`
		Activity  uint32 `json:"activity"`
		ID        uint32 `json:"id"`
		Group     bool   `json:"group"`
		Refreshes bool   `json:"refreshes"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.AdminResetShop(ctx, id, b.Kind, b.Activity, b.ID, b.Group, b.Refreshes)
	})
}

// handleAkashi sets Akashi's affection, or forgets it (reset).
func (server *Server) handleAkashi(r *http.Request) (any, error) {
	type body struct {
		Exp   uint32 `json:"exp"`
		Taps  uint32 `json:"taps"`
		Reset bool   `json:"reset"`
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
		return server.Game.AdminSetAkashi(ctx, id, b.Exp, b.Taps, b.Reset)
	})
}

// handleRequisitionsReset forgets this month's requisitions.
func (server *Server) handleRequisitionsReset(r *http.Request) (any, error) {
	return commanderEdit(server, r, func(ctx context.Context, id uint32, _ struct{}) (bool, error) {
		return server.Game.AdminResetRequisitions(ctx, id)
	})
}

// --- activities

type lifeTable struct {
	Table   string   `json:"table"`
	Label   string   `json:"label"`
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
	Total   int      `json:"total"`
}

type lifeActivity struct {
	Key      string      `json:"key"`
	Name     string      `json:"name"`
	Kind     string      `json:"kind"`
	Category string      `json:"category"`
	Activity uint32      `json:"activity,omitempty"`
	Type     uint32      `json:"type,omitempty"`
	Status   string      `json:"status"`
	Start    int64       `json:"start"`
	End      int64       `json:"end"`
	Progress string      `json:"progress"`
	Tasks    int         `json:"tasks"`
	Tables   []lifeTable `json:"tables"`
}

// systemNames name game.AdminStates' systems: what the screen calls them
// and their category.
var systemNames = map[string][2]string{
	"atelier":     {"Atelier", "build"},
	"juus":        {"Juustagram", "social"},
	"apartment":   {"Private Quarters", "social"},
	"loveletters": {"Love Letters", "social"},
	"educate":     {"Project Identity: TB", "story"},
	"reflux":      {"Returning Commander", "return"},
	"remaster":    {"Capsule Rerun", "rerun"},
	"minigames":   {"Mini-games", "minigame"},
	"arcade":      {"Arcade", "minigame"},
	"playroom":    {"Play room", "minigame"},
	"island":      {"Island", "island"},
}

// activityCategory sorts the activity types into the screen's few colours.
func activityCategory(kind uint32) string {
	switch kind {
	case 3, 6, 8, 109, 141: // logins and sign-ins
		return "login"
	case 13, 18, 39, 40, 44, 54, 82, 86, 89, 110: // tasks and passes
		return "tasks"
	case 14, 27, 48, 83, 84, 119, 120, 140: // shops
		return "shop"
	case 21, 33, 43, 45, 50, 51, 57, 61, 72, 75, 78, 92, 98, 102, 105, 136, 137, 1001:
		return "minigame"
	case 1, 26, 55, 80, 85, 88, 96, 129:
		return "build"
	case 95, 800, 801, 890, 999:
		return "island"
	case 42:
		return "return"
	case 29, 60, 104, 133, 134:
		return "social"
	}
	return "event"
}

// maxStateRows is how many rows of a table the detail card gets.
const maxStateRows = 200

// cell is a db value as the screen shows it.
func cell(value reflect.Value) any {
	switch v := value.Interface().(type) {
	case []byte:
		return fmt.Sprintf("%d bytes", len(v))
	case string:
		if len(v) > 300 {
			return v[:300] + "…"
		}
		return v
	case bool, int64, uint32, int32, float64:
		return v
	case db.IDList, db.Drops, db.Cells:
		raw, _ := json.Marshal(v)
		return string(raw)
	}
	raw, err := json.Marshal(value.Interface())
	if err != nil {
		return fmt.Sprint(value.Interface())
	}
	return string(raw)
}

// stateTable turns db rows into columns and cells, the commander and the
// activity id left out.
func stateTable(table game.AdminStateTable) lifeTable {
	out := lifeTable{Table: table.Table, Label: humanize(strings.TrimPrefix(table.Table, "commander_")), Columns: []string{}, Rows: [][]any{}, Total: len(table.Rows)}
	if len(table.Rows) == 0 {
		return out
	}
	kind := reflect.TypeOf(table.Rows[0])
	var fields []int
	for i := range kind.NumField() {
		switch name := kind.Field(i).Name; name {
		case "CommanderID", "ActivityID", "ActID":
		default:
			fields = append(fields, i)
			out.Columns = append(out.Columns, snake(name))
		}
	}
	for _, row := range table.Rows[:min(len(table.Rows), maxStateRows)] {
		value := reflect.ValueOf(row)
		cells := make([]any, 0, len(fields))
		for _, i := range fields {
			cells = append(cells, cell(value.Field(i)))
		}
		out.Rows = append(out.Rows, cells)
	}
	return out
}

// snake is a Go field name as its column (Data1List: data1_list).
func snake(name string) string {
	var out []rune
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 && !unicode.IsUpper(rune(name[i-1])) {
				out = append(out, '_')
			}
			r = unicode.ToLower(r)
		}
		out = append(out, r)
	}
	return string(out)
}

// handleActivities lists every activity and system the commander keeps
// state of, with the stored rows.
func (server *Server) handleActivities(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	tables, err := loadLifeTables()
	if err != nil {
		return nil, err
	}
	states, err := server.Game.AdminStates(r.Context(), commander.ID)
	if err != nil {
		return nil, err
	}
	list := make([]lifeActivity, 0, len(states))
	for _, state := range states {
		row := lifeActivity{Key: state.Key, Activity: state.Activity, Type: state.Type, Status: state.Status, Start: state.Start, End: state.End, Tables: []lifeTable{}}
		switch {
		case state.Activity != 0:
			row.Kind = activityKinds[state.Type]
			if row.Kind == "" {
				row.Kind = "Activity"
			}
			row.Category = activityCategory(state.Type)
			row.Name = tables.activities[state.Activity]
			if row.Name == "" {
				row.Name = row.Kind
			}
		case state.Character != 0:
			row.Name, row.Kind, row.Category = "Project Identity: "+tables.children[state.Character], "Project Identity", "story"
			if tables.children[state.Character] == "" {
				row.Name = fmt.Sprintf("Project Identity #%d", state.Character)
			}
		default:
			names := systemNames[state.Key]
			row.Name, row.Kind, row.Category = names[0], names[0], names[1]
		}
		var parts []string
		for _, table := range state.Tables {
			row.Tables = append(row.Tables, stateTable(table))
			if table.Table == "commander_activity_tasks" {
				row.Tasks = len(table.Rows)
				finished := 0
				for _, task := range table.Rows {
					if task.(db.CommanderActivityTask).Finished {
						finished++
					}
				}
				parts = append(parts, fmt.Sprintf("%d of %d tasks done", finished, len(table.Rows)))
				continue
			}
			if len(parts) < 2 && len(table.Rows) > 1 {
				parts = append(parts, fmt.Sprintf("%d %s", len(table.Rows), strings.ToLower(humanize(lastWord(table.Table)))))
			}
		}
		row.Progress = strings.Join(parts, " · ")
		list = append(list, row)
	}
	order := map[string]int{game.AdminStateRunning: 0, game.AdminStatePermanent: 1, game.AdminStateFinished: 2}
	sort.SliceStable(list, func(i, j int) bool { return order[list[i].Status] < order[list[j].Status] })
	return list, nil
}

func lastWord(table string) string {
	return table[strings.LastIndex(table, "_")+1:]
}

// handleActivityClear deletes the commander's rows of one activity or
// system (game.AdminClearState), or only its task progress.
func (server *Server) handleActivityClear(tasksOnly bool) func(*http.Request) (any, error) {
	type body struct {
		Key string `json:"key"`
	}
	return func(r *http.Request) (any, error) {
		return commanderEdit(server, r, func(ctx context.Context, id uint32, b body) (bool, error) {
			return server.Game.AdminClearState(ctx, id, b.Key, tasksOnly)
		})
	}
}

// --- archives

type lifeArchiveShip struct {
	Group    uint32 `json:"group"`
	Name     string `json:"name"`
	Icon     string `json:"icon,omitempty"`
	Rarity   uint32 `json:"rarity"`
	Level    uint32 `json:"level"`
	Intimacy uint32 `json:"intimacy"`
	Stars    uint32 `json:"stars"`
	Married  bool   `json:"married"`
	Retrofit bool   `json:"retrofit"`
}

type lifeMedal struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Progress uint32 `json:"progress"`
	Target   uint32 `json:"target"`
	Claimed  int    `json:"claimed"`
	Tiers    int    `json:"tiers"`
	Shown    bool   `json:"shown"`
}

type lifeCollection struct {
	ID      uint32 `json:"id"`
	Name    string `json:"name"`
	Claimed int    `json:"claimed"`
	Total   int    `json:"total"`
}

type lifeHandbook struct {
	ID          uint32 `json:"id"`
	Name        string `json:"name"`
	Done        int    `json:"done"`
	Tasks       int    `json:"tasks"`
	Awards      uint32 `json:"awards"`
	AwardsTotal uint32 `json:"awards_total"`
}

type lifeSeen struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int64  `json:"count"`
	Liked int64  `json:"liked"`
}

type lifeOption struct {
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Icon   string `json:"icon,omitempty"`
	Rarity uint32 `json:"rarity,omitempty"`
	Level  uint32 `json:"level,omitempty"`
}

type lifeLook struct {
	Secretaries    []uint32 `json:"secretaries"`
	RandomMode     uint32   `json:"random_mode"`
	RandomEnabled  bool     `json:"random_enabled"`
	MarryShip      uint32   `json:"marry_ship"`
	ChildDisplay   uint32   `json:"child_display"`
	IconFrame      uint32   `json:"icon_frame"`
	ChatFrame      uint32   `json:"chat_frame"`
	Cover          uint32   `json:"cover"`
	MaxSecretaries int      `json:"max_secretaries,omitempty"`
}

type lifeArchives struct {
	Ships       []lifeArchiveShip `json:"ships"`
	Medals      []lifeMedal       `json:"medals"`
	Displayed   int               `json:"displayed"`
	Collections []lifeCollection  `json:"collections"`
	Handbook    []lifeHandbook    `json:"handbook"`
	Seen        []lifeSeen        `json:"seen"`
	Music       struct {
		Album uint32 `json:"album"`
		Mode  uint32 `json:"mode"`
	} `json:"music"`
	Look    lifeLook `json:"look"`
	Options struct {
		Ships      []lifeOption `json:"ships"` // every ship of the commander
		Oath       []lifeOption `json:"oath"`  // its oathed ships
		Specials   []lifeOption `json:"specials"`
		IconFrames []lifeOption `json:"icon_frames"`
		ChatFrames []lifeOption `json:"chat_frames"`
		Covers     []lifeOption `json:"covers"`
	} `json:"options"`
}

// shipGroupName is a ship_data_template group's name and icon: its first
// template's.
func (server *Server) shipGroupName(group uint32) named {
	for star := uint32(1); star <= 9; star++ {
		if row, ok := server.names[dropShip][group*10+star]; ok {
			row.Name = strings.TrimSpace(row.Name)
			return row
		}
	}
	return named{Name: fmt.Sprintf("#%d", group)}
}

// handleArchives is the commander's archives: the ship archive, medals,
// collection milestones, manual pages, what it saw and unlocked, and its
// look with what it may pick from.
func (server *Server) handleArchives(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	ctx, id := r.Context(), commander.ID
	tables, err := loadLifeTables()
	if err != nil {
		return nil, err
	}
	reply := lifeArchives{Ships: []lifeArchiveShip{}, Medals: []lifeMedal{}, Collections: []lifeCollection{}, Handbook: []lifeHandbook{}, Seen: []lifeSeen{}}

	groups, err := server.Game.AdminShipGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		named := server.shipGroupName(group.GroupID)
		reply.Ships = append(reply.Ships, lifeArchiveShip{
			Group: group.GroupID, Name: named.Name, Icon: named.Icon, Rarity: named.Rarity,
			Level: group.LvMax, Intimacy: group.IntimacyMax, Stars: group.Stars, Married: group.MarryFlag != 0, Retrofit: group.Trans,
		})
	}
	sort.SliceStable(reply.Ships, func(i, j int) bool {
		a, b := reply.Ships[i], reply.Ships[j]
		if a.Level != b.Level {
			return a.Level > b.Level
		}
		return a.Group < b.Group
	})

	displayed, err := server.Store.ListDisplayMedals(ctx, id)
	if err != nil {
		return nil, err
	}
	reply.Displayed = len(displayed)
	medals, err := server.Game.AdminMedals(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, medal := range medals {
		shown := 0
		if medal.Claimed < len(medal.Tiers) {
			shown = medal.Claimed
		} else {
			shown = len(medal.Tiers) - 1
		}
		row := lifeMedal{ID: medal.ID, Name: tables.medals[medal.ID].Name, Progress: medal.Progress, Target: medal.Targets[shown], Claimed: medal.Claimed, Tiers: len(medal.Tiers)}
		for _, tier := range medal.Tiers {
			for _, display := range displayed {
				row.Shown = row.Shown || display == tier
			}
		}
		reply.Medals = append(reply.Medals, row)
	}

	awards, err := server.Store.ListCollectionAwards(ctx, id)
	if err != nil {
		return nil, err
	}
	claimed := map[uint32]int{}
	for _, award := range awards {
		claimed[award.StoreupID]++
	}
	for storeup, row := range tables.storeups {
		reply.Collections = append(reply.Collections, lifeCollection{ID: storeup, Name: row.Name, Claimed: claimed[storeup], Total: len(row.AwardDisplay)})
	}
	sort.Slice(reply.Collections, func(i, j int) bool { return reply.Collections[i].ID < reply.Collections[j].ID })

	pages, err := server.Game.AdminHandbook(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, page := range pages {
		reply.Handbook = append(reply.Handbook, lifeHandbook{ID: page.ID, Name: tables.handbookTasks[page.ID], Done: page.Done, Tasks: page.Tasks, Awards: page.Awards, AwardsTotal: page.AwardsTotal})
	}

	seen, err := server.Game.AdminSeen(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, entry := range []struct{ key, label string }{
		{game.AdminSeenStories, "Stories seen"},
		{game.AdminSeenFlags, "Client flags"},
		{game.AdminSeenGallery, "Gallery pictures"},
		{game.AdminSeenManga, "Manga chapters"},
		{game.AdminSeenMusic, "Songs"},
	} {
		counts := seen[entry.key]
		reply.Seen = append(reply.Seen, lifeSeen{Key: entry.key, Label: entry.label, Count: counts[0], Liked: counts[1]})
	}

	look, err := server.Game.AdminGetLook(ctx, id)
	if err != nil {
		return nil, err
	}
	reply.Music.Album, reply.Music.Mode = look.MusicNo, look.MusicM
	reply.Look = lifeLook{
		Secretaries: look.Secretaries, RandomMode: look.RandomMode, RandomEnabled: look.RandomEnabled, MarryShip: look.MarryShip,
		ChildDisplay: look.ChildDisplay, IconFrame: look.IconFrame, ChatFrame: look.ChatFrame, Cover: look.Cover, MaxSecretaries: look.MaxSecretaries,
	}
	if reply.Look.Secretaries == nil {
		reply.Look.Secretaries = []uint32{}
	}
	ships, err := server.Store.ListShipsByCommander(ctx, id)
	if err != nil {
		return nil, err
	}
	reply.Options.Ships, reply.Options.Oath = []lifeOption{}, []lifeOption{}
	for _, ship := range ships {
		template := server.names[dropShip][ship.TemplateID]
		name := strings.TrimSpace(ship.Name)
		if name == "" {
			name = strings.TrimSpace(template.Name)
		}
		option := lifeOption{ID: ship.ID, Name: name, Icon: server.names.shipIcon(ship.TemplateID, ship.SkinID), Rarity: template.Rarity, Level: ship.Level}
		reply.Options.Ships = append(reply.Options.Ships, option)
		if ship.Propose != 0 {
			reply.Options.Oath = append(reply.Options.Oath, option)
		}
	}
	// icon is the option's bundle (IconFrame.GetIcon, LivingAreaCover.GetIcon);
	// chat frames' prefabs hold no texture of their own, so they have none
	options := func(ids []uint32, names map[uint32]string, icon func(uint32) string) []lifeOption {
		list := []lifeOption{}
		seen := map[string]int{}
		for _, id := range ids {
			seen[names[id]]++
		}
		for _, id := range ids {
			name := strings.TrimSpace(names[id])
			if name == "" || seen[names[id]] > 1 {
				name = strings.TrimSpace(fmt.Sprintf("%s #%d", name, id))
			}
			list = append(list, lifeOption{ID: id, Name: name, Icon: icon(id)})
		}
		return list
	}
	none := func(uint32) string { return "" }
	reply.Options.Specials = options(look.SpecialOptions, tables.specials, none)
	reply.Options.IconFrames = options(look.IconFrames, tables.iconFrames, func(id uint32) string { return fmt.Sprintf("iconframe/%d", id) })
	reply.Options.ChatFrames = options(look.ChatFrames, tables.chatFrames, none)
	reply.Options.Covers = options(look.Covers, tables.covers, func(id uint32) string { return tables.coverIcons[id] })
	return reply, nil
}

// handleMedalReset forgets the tiers claimed of a medal.
func (server *Server) handleMedalReset(r *http.Request) (any, error) {
	var medal uint32
	if _, err := fmt.Sscan(r.PathValue("medal"), &medal); err != nil {
		return nil, badRequest("medal: want an id")
	}
	return commanderEdit(server, r, func(ctx context.Context, id uint32, _ struct{}) (bool, error) {
		return server.Game.AdminResetMedal(ctx, id, medal)
	})
}

// handleSeenReset clears one list of what the commander saw or unlocked.
func (server *Server) handleSeenReset(r *http.Request) (any, error) {
	return commanderEdit(server, r, func(ctx context.Context, id uint32, _ struct{}) (bool, error) {
		return server.Game.AdminResetSeen(ctx, id, r.PathValue("what"))
	})
}

// handleLook saves the commander's look.
func (server *Server) handleLook(r *http.Request) (any, error) {
	return commanderEdit(server, r, func(ctx context.Context, id uint32, look lifeLook) (bool, error) {
		return server.Game.AdminSetLook(ctx, id, game.AdminLook{
			Secretaries: look.Secretaries, RandomMode: look.RandomMode, RandomEnabled: look.RandomEnabled, MarryShip: look.MarryShip,
			ChildDisplay: look.ChildDisplay, IconFrame: look.IconFrame, ChatFrame: look.ChatFrame, Cover: look.Cover,
		})
	})
}
