package game

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// item_data_statistics usage (ItemUsage, model/const/itemusage.lua) of the
// items belfast opens. The others change state belfast doesn't keep yet
// (skins, ship affection gifts, the backyard level, the guild, commander
// boxes, remastered chapters) and are refused.
const (
	usageDrop          = "usage_drop"
	usageDropTemplate  = "usgae_drop_template" // sic
	usageDropAppointed = "usage_drop_appointed"
	usageInvitation    = "usage_invitation"
	usageFood          = "usage_food"
)

// itemUseConfig is what item_data_statistics says about using an item.
type itemUseConfig struct {
	Type        uint32          `json:"type"`
	Name        string          `json:"name"`
	Display     string          `json:"display"`
	Usage       string          `json:"usage"`
	UsageArg    json.RawMessage `json:"usage_arg"`
	DisplayIcon json.RawMessage `json:"display_icon"`
}

// itemUse is an item_data_statistics usage with its usage_arg read.
type itemUse struct {
	usage string
	// usage_drop, usgae_drop_template: what one item gives. The drop
	// tables are server-side; display_icon lists the contents the client
	// shows, and a template's usage_arg [drop, gold, oil] its resources
	// (Item:GetOverflowCheckItems, model/vo/item.lua).
	contents [][]uint32
	// usage_drop tech packs without a drop_packs.json entry: the fallback
	// draw
	pack dropPack
	// usage_drop_appointed: the [type, id, count] entries one is picked
	// from (AssignedItemView, view/equipment/assigneditemview.lua).
	choices [][]uint32
	// usage_invitation: the item_usage_invitation ids one is picked from
	// (BaseAssignedShipScene, view/activity/assignedship/).
	invitations []uint32
	// usage_food: the food one item adds to the backyard (AddFoodCommand)
	food uint32
}

var itemUses map[uint32]itemUse

// item_usage_invitation row: the ship an invitation gives, and the
// commemorative item that comes with it (0 none)
type invitationConfig struct {
	ShipID uint32 `json:"ship_id"`
	CoinID uint32 `json:"coin_id"`
}

var invitations map[uint32]invitationConfig

// dormFoodCapacity is dorm_data_template[1].capacity, the food the level 1
// backyard holds (no food_max_increase extension is sold yet).
var dormFoodCapacity uint32

// dorm_data_template row
type dormConfig struct {
	Capacity uint32 `json:"capacity"`
}

func loadItemData() error {
	var dorms map[uint32]dormConfig
	err := gamedata.LoadAll(
		gamedata.Load("item_usage_invitation", &invitations),
		gamedata.Load("dorm_data_template", &dorms),
		loadDropPacks,
	)
	if err != nil {
		return err
	}
	dorm, ok := dorms[1]
	if !ok {
		return gamedata.Missing("dorm_data_template", "level 1")
	}
	dormFoodCapacity = dorm.Capacity
	return nil
}

// itemTypeEquipBox is Item.EQUIPMENT_BOX_TYPE_5 (model/vo/item.lua), the
// tech packs (the client checks the depot has room for count gear).
const itemTypeEquipBox = 5

// packGroup is one outcome of a random pack: drawn by weight, then one of
// its pool's [type, id, count] entries.
type packGroup struct {
	Weight uint32     `json:"weight"`
	Pool   [][]uint32 `json:"pool"`
}

// dropPack is a random pack's draw.
type dropPack []packGroup

// dropPacks are what the packs whose drop tables are server-side hold, by
// item_data_statistics or item_virtual_data_statistics id: the tech packs,
// the gear development packs, the "Mystery" placeholders the gift packs'
// display_icon lists (54033 Mystery T3 Tech Pack...). Server content from
// server/drop_packs.json of the data dir ({"<id>": {"groups": [...]}}),
// scraped from the wiki's equipment box table by
// internal/tools/wiki_drop_packs.py. Without it the nation tech packs fall
// back to what their item text says (readTechPack).
var dropPacks map[uint32]dropPack

func loadDropPacks() error {
	dropPacks = map[uint32]dropPack{}
	raw, err := gamedata.ServerFile("drop_packs.json")
	if raw == nil || err != nil {
		return err
	}
	var rows map[uint32]struct {
		Groups dropPack `json:"groups"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("drop packs: %w", err)
	}
	for id, row := range rows {
		if row.Groups.valid() {
			dropPacks[id] = row.Groups
		}
	}
	return nil
}

func (pack dropPack) valid() bool {
	return len(pack) > 0 && !slices.ContainsFunc(pack, func(group packGroup) bool {
		return group.Weight == 0 || len(group.Pool) == 0 || slices.ContainsFunc(group.Pool, func(entry []uint32) bool { return len(entry) < 3 })
	})
}

// draw draws the pack count times, merging repeats.
func (pack dropPack) draw(count uint32) []*protobuf.DROPINFO {
	var drops []*protobuf.DROPINFO
	for range count {
		i := weightedPick(rand.Uint32N, pack, func(group packGroup) uint32 { return group.Weight })
		if i < 0 {
			return nil
		}
		entry := pack[i].Pool[rand.IntN(len(pack[i].Pool))]
		drops = mergeDrop(drops, entry[0], entry[1], entry[2])
	}
	return drops
}

// mergeDrop adds a reward to drops, to its entry if it has one (ships
// excepted: each is its own).
func mergeDrop(drops []*protobuf.DROPINFO, kind, id, number uint32) []*protobuf.DROPINFO {
	if kind != dropTypeShip {
		if i := slices.IndexFunc(drops, func(drop *protobuf.DROPINFO) bool { return drop.GetType() == kind && drop.GetId() == id }); i >= 0 {
			drops[i].Number = proto.Uint32(drops[i].GetNumber() + number)
			return drops
		}
	}
	return append(drops, dropInfo(kind, id, number))
}

// packGear is a piece of gear a tech pack can hold: a +0 equip_data_template
// whose statistics have a grade (tech 1-3).
type packGear struct {
	id, nationality, rarity uint32
}

func techPackGear(templates map[uint32]equipTemplateConfig, stats map[uint32]equipStatisticsConfig) []packGear {
	var gear []packGear
	for id, template := range templates {
		stat, ok := stats[id]
		if ok && template.Prev == 0 && stat.Tech > 0 {
			gear = append(gear, packGear{id, stat.Nationality, stat.Rarity})
		}
	}
	slices.SortFunc(gear, func(a, b packGear) int { return cmp.Compare(a.id, b.id) })
	return gear
}

// nationTips are the gametip names of the nations (Nation.Nation2Name,
// model/const/nation.lua) a tech pack can be of.
var nationTips = map[string]uint32{
	"word_shipNation_baiYing":   1,
	"word_shipNation_huangJia":  2,
	"word_shipNation_chongYing": 3,
	"word_shipNation_tieXue":    4,
	"word_shipNation_dongHuang": 5,
	"word_shipNation_saDing":    6,
	"word_shipNation_beiLian":   7,
	"word_shipNation_ziyou":     8,
	"word_shipNation_weixi":     9,
}

// packRatePatterns read a tech pack's display lines once their color tags
// are stripped: "40% chance to receive Rare gear.", and the one rarity
// packs: "Contains Common gear", "Guaranteed to receive Super Rare gear".
var (
	packRatePattern = regexp.MustCompile(`(\d+)% chance to receive (Super Rare|Common|Rare|Elite) gear`)
	packOnlyPattern = regexp.MustCompile(`(?:Contains|Guaranteed to receive) (Super Rare|Common|Rare|Elite) gear`)
)

// readTechPack is the fallback draw of a tech pack, from what the client
// shows of it: a rarity by the rates of its display text ("40% chance to
// receive Rare gear"), then any graded +0 gear of that rarity of the nation
// whose name's first word is a word of the item's name ("T2 Sakura Tech
// Pack"). nil when either can't be read (the starter submarine box, the
// gear development packs).
func readTechPack(row itemConfig, gear []packGear, gametip map[string]string) dropPack {
	words := strings.Fields(row.Name)
	nationality, found := uint32(0), 0
	for key, nation := range nationTips {
		if name := strings.Fields(gametip[key]); len(name) > 0 && slices.Contains(words, name[0]) {
			nationality, found = nation, found+1
		}
	}
	if found != 1 {
		return nil
	}
	byRarity := map[uint32][][]uint32{}
	for _, piece := range gear {
		if piece.nationality == nationality {
			byRarity[piece.rarity] = append(byRarity[piece.rarity], []uint32{dropTypeEquip, piece.id, 1})
		}
	}
	display := rateTipTags.ReplaceAllString(row.Display, "")
	var pack dropPack
	for _, match := range packRatePattern.FindAllStringSubmatch(display, -1) {
		rate, err := strconv.ParseUint(match[1], 10, 32)
		if err == nil {
			pack = append(pack, packGroup{uint32(rate), byRarity[rarityByRateName[match[2]]]})
		}
	}
	if match := packOnlyPattern.FindStringSubmatch(display); len(pack) == 0 && match != nil {
		pack = dropPack{{100, byRarity[rarityByRateName[match[1]]]}}
	}
	if !pack.valid() {
		return nil
	}
	return pack
}

// readItemUses reads the usage of the item_data_statistics rows belfast
// opens.
func readItemUses(items map[uint32]itemConfig, gear []packGear, gametip map[string]string) {
	itemUses = make(map[uint32]itemUse, len(items))
	for id, row := range items {
		use := itemUse{usage: row.Usage}
		var icons, args [][]uint32
		_ = json.Unmarshal(row.DisplayIcon, &icons)
		switch row.Usage {
		case usageDrop:
			use.contents = icons
			if len(icons) == 0 && row.Type == itemTypeEquipBox {
				use.pack = readTechPack(row, gear, gametip)
			}
		case usageDropTemplate:
			var template []uint32
			if json.Unmarshal(row.UsageArg, &template) != nil || len(template) < 3 {
				break
			}
			if template[1] > 0 {
				use.contents = append(use.contents, []uint32{dropTypeResource, ResourceGold, template[1]})
			}
			if template[2] > 0 {
				use.contents = append(use.contents, []uint32{dropTypeResource, ResourceOil, template[2]})
			}
			// display_icon shows the template's gold and oil too
			for _, icon := range icons {
				if len(icon) >= 2 && icon[0] == dropTypeResource && (icon[1] == ResourceGold || icon[1] == ResourceOil) {
					continue
				}
				use.contents = append(use.contents, icon)
			}
		case usageDropAppointed:
			if json.Unmarshal(row.UsageArg, &args) == nil {
				use.choices = args
			}
		case usageInvitation:
			use.invitations = gamedata.IDs(row.UsageArg)
		case usageFood:
			if food := gamedata.IDs(row.UsageArg); len(food) > 0 {
				use.food = food[0]
			}
		}
		itemUses[id] = use
	}
}

// storableDrop is whether grantDrops keeps a reward of an item's contents:
// resources, bag items, equipment and ships. display_icon also lists
// placeholders for what the server draws (the "Mystery" virtual items,
// random skins) and kinds belfast doesn't store (skins, gear skins,
// furniture, frames...).
func storableDrop(kind, id uint32) bool {
	switch kind {
	case dropTypeResource:
		return true
	case dropTypeItem:
		_, ok := itemUses[id]
		return ok
	case dropTypeEquip:
		return equipIDs[id]
	case dropTypeShip:
		_, ok := shipTemplates[id]
		return ok
	}
	return false
}

// handleUseItem answers UseItemCommand and UseFudaiItemCommand
// (controller/command/item/), using count of a bag item, arg naming the pick
// of a selection box. On result 0 the client removes the items itself and,
// for the boxes and invitations, adds drop_list (PlayerConst.addTranDrop);
// for food it adds usage_arg[1] per item to the backyard itself
// (AddFoodCommand) and ignores drop_list. No official capture has it: the
// reply is built from the Lua alone.
func (game *Game) handleUseItem(client *connection.Client, req *protobuf.CS_15002) error {
	ctx := context.Background()
	var drops []*protobuf.DROPINFO
	err := game.tx(ctx, client, func(tx *Tx) (err error) {
		drops, err = tx.useItem(req.GetId(), req.GetCount(), req.GetArg())
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		drops = nil
	}
	return client.Send(&protobuf.SC_15003{Result: result(err), DropList: drops})
}

// useItem spends count of an item and gives what it holds, returning the
// drops given. Nothing is spent when it can't be opened.
func (tx *Tx) useItem(id, count uint32, arg []uint32) ([]*protobuf.DROPINFO, error) {
	use, ok := itemUses[id]
	if !ok || count == 0 {
		return nil, errRefused
	}
	var drops []*protobuf.DROPINFO
	switch use.usage {
	case usageDrop, usageDropTemplate:
		if pack, ok := dropPacks[id]; ok {
			drops = pack.draw(count)
			break
		}
		if use.pack != nil {
			drops = use.pack.draw(count)
			break
		}
		if len(use.contents) == 0 {
			return nil, errRefused
		}
		for _, entry := range use.contents {
			if len(entry) < 3 {
				return nil, errRefused
			}
			drops = resolveContent(drops, entry[0], entry[1], entry[2]*count)
		}
	case usageDropAppointed:
		// arg is the picked usage_arg entry
		i := slices.IndexFunc(use.choices, func(choice []uint32) bool { return slices.Equal(choice, arg) })
		if i < 0 || len(arg) < 3 {
			return nil, errRefused
		}
		drops = []*protobuf.DROPINFO{dropInfo(arg[0], arg[1], arg[2]*count)}
	case usageInvitation:
		// arg is the picked item_usage_invitation id
		if len(arg) != 1 || !slices.Contains(use.invitations, arg[0]) {
			return nil, errRefused
		}
		invitation, ok := invitations[arg[0]]
		if !ok {
			return nil, errRefused
		}
		drops = []*protobuf.DROPINFO{dropInfo(dropTypeShip, invitation.ShipID, count)}
		if invitation.CoinID != 0 {
			drops = append(drops, dropInfo(dropTypeItem, invitation.CoinID, count))
		}
	case usageFood:
		if use.food == 0 {
			return nil, errRefused
		}
		if err := tx.SpendItem(id, count); err != nil {
			return nil, err
		}
		return nil, tx.addDormFood(use.food * count)
	default:
		return nil, errRefused
	}
	if len(drops) == 0 {
		return nil, errRefused
	}
	for _, drop := range drops {
		if !storableDrop(drop.GetType(), drop.GetId()) {
			return nil, errRefused
		}
	}
	if err := tx.SpendItem(id, count); err != nil {
		return nil, err
	}
	return drops, tx.grantDrops(drops)
}

// resolveContent adds number of a pack's content to drops. A display_icon
// item entry that isn't an item is a placeholder for what the server draws:
// a "Mystery" one of dropPacks, or a container virtual item whose own
// display_icon lists what it turns into (dropPools: 150001 T2 Development
// Materials Container...), drawn once per unit. What can't be resolved is
// left as is, for storableDrop to refuse.
func resolveContent(drops []*protobuf.DROPINFO, kind, id, number uint32) []*protobuf.DROPINFO {
	if _, item := itemUses[id]; kind != dropTypeItem || item {
		return mergeDrop(drops, kind, id, number)
	}
	if pack, ok := dropPacks[id]; ok {
		for _, drop := range pack.draw(number) {
			drops = mergeDrop(drops, drop.GetType(), drop.GetId(), drop.GetNumber())
		}
		return drops
	}
	if pool := dropPools[id]; len(pool) > 0 {
		for range number {
			ref := pool[rand.IntN(len(pool))]
			drops = mergeDrop(drops, ref.kind, ref.id, 1)
		}
		return drops
	}
	return mergeDrop(drops, kind, id, number)
}

// addDormFood fills the backyard with food, what doesn't fit is lost
// (AddFoodCommand).
func (tx *Tx) addDormFood(food uint32) error {
	commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	return tx.SetDormFood(tx.ctx, db.SetDormFoodParams{DormFood: uint32(min(uint64(commander.DormFood)+uint64(food), uint64(dormFoodCapacity))), ID: tx.commanderID})
}
