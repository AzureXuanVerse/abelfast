package game

import (
	"context"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Operation Siren's Operational Handover (ChapterAutoProxy.TYPE.WORLD, the
// atlas' delegate button and a sector's, ChapterAutoPanelTypeWorld /
// ChapterAutoPanelTypeWorldSingle): unsecured sectors the atlas reaches are
// "secured" in the background, one after the other, each for its
// world_auto_statistics row's oil_limit action points and time_correction
// seconds. Claiming it (CS_13014, EndChapterAutoCommand's WORLD branches)
// marks the finished ones secured by auto-search and gives their rewards;
// the unfinished ones give their action points back. It shares the
// campaign handover's sortie list (chapter_auto_battles, type 2, id the
// map) and its claim. Nothing was captured: built from the Lua and the
// tables alone, check on the device first.

// ChapterAutoProxy.TYPE.WORLD
const chapterAutoWorld = 2

// worldAutoConfig is a world_auto_statistics row, a sector the handover
// can secure (its id the map, world_chapter_random).
type worldAutoConfig struct {
	OilLimit       uint32 `json:"oil_limit"`       // action points (ChapterAutoCommission.GetOnceOil)
	TimeCorrection uint32 `json:"time_correction"` // seconds it takes
	// ExpBook is what the panel shows under "Lecture Hall Proficiency"
	// (uiLeftProficiencyText); the claim's class_exp
	ExpBook uint32 `json:"drop_expbook"`
}

var (
	worldAutoConfigs map[uint32]worldAutoConfig
	// gameset world_auto_battle_map_limit: sectors a handover takes at most
	worldAutoMapLimit uint32
	// gameset world_auto_finish_map_limit's description: achievements
	// (world_target_data, "Gain control of 1 sector(s)" of the corrosion 6
	// sectors), one of which opens the handover (World:CanDelegate)
	worldAutoFinishTargets []uint32
)

func loadWorldAutoData() error {
	var gameset map[string]uint32
	err := gamedata.LoadAll(
		gamedata.Load("world_auto_statistics", &worldAutoConfigs),
		func() (err error) {
			gameset, err = gamedata.Gameset()
			return err
		},
		func() error {
			return gamedata.GamesetDescription("world_auto_finish_map_limit", &worldAutoFinishTargets)
		},
	)
	if err != nil {
		return err
	}
	limit, ok := gameset["world_auto_battle_map_limit"]
	if !ok {
		return gamedata.Missing("gameset", "%s", "world_auto_battle_map_limit")
	}
	worldAutoMapLimit = limit
	return nil
}

// transportEntrances is WorldAtlas.transportDic, the entrances the atlas
// lets the fleets sail to, as the client rebuilds it from the full state:
// an entrance whose base map is secured (clean_chapter) and the ones it
// links to (map_transfer, BuildTransportDic), every port's entrances once
// the world was reset (AddPortTransportDic: activated more than once), and
// the entrances whose zone securing reward was taken (chapter_award flag
// 0, NetUpdateWorldPressingAward's MarkMapTransport).
func (state *worldState) transportEntrances() map[uint32]bool {
	open := map[uint32]bool{}
	reset := state.Count.Activations > 1
	for id, mask := range worldColormasks {
		if slices.Contains(state.CleanChapters, mask.Chapter) {
			open[id] = true
			for _, linked := range mask.MapTransfer {
				open[linked] = true
			}
		}
		if reset && mask.PortMapIcon > 0 {
			open[id] = true
		}
	}
	for _, award := range state.ChapterAwards {
		if award.Flag == 0 {
			open[award.ID] = true
		}
	}
	return open
}

// mapEntrance is WorldAtlas.mapEntrance: the entrance whose base map is
// randomID, 0 for none.
func mapEntrance(randomID uint32) uint32 {
	if mask, ok := worldColormasks[randomID]; ok && mask.Chapter == randomID {
		return randomID
	}
	for id, mask := range worldColormasks {
		if mask.Chapter == randomID {
			return id
		}
	}
	return 0
}

// delegable is World:GetDelegateMapDic for one map: a world_auto_statistics
// sector not in chapter_list (isCost: neither entered nor delegated yet)
// whose entrance the atlas reaches.
func (state *worldState) delegable(randomID uint32, transport map[uint32]bool) bool {
	if _, ok := worldAutoConfigs[randomID]; !ok || state.mapTemplate(randomID) != 0 {
		return false
	}
	entrance := mapEntrance(randomID)
	return entrance != 0 && transport[entrance]
}

// canDelegate is World:CanDelegate without the running handover: the
// treasure_flag global flag (the Hidden Zone Data Logger used) and one of
// world_auto_finish_map_limit's achievements.
func (state *worldState) canDelegate() bool {
	if len(state.GlobalFlags) == 0 || state.GlobalFlags[0] == 0 {
		return false
	}
	return slices.ContainsFunc(worldAutoFinishTargets, state.achieved)
}

// handleStartWorldChapterAuto starts an Operation Siren handover
// (StartWorldChapterAutoCommand, CS_13018): the sectors map_id_list, in
// that order (the panel's, by corrosion then id). The client checks, and
// the server again: the world is activated, no handover is running
// (World:CanDelegate), treasure_flag is set and one of the
// world_auto_finish_map_limit achievements achieved, at most
// world_auto_battle_map_limit sectors, each once and delegable
// (GetDelegateMapDic), and the action points for them all (oil_limit each,
// WorldStaminaManager:GetTotalStamina). The panel never uses a permit
// (needTicket is always false; the request has no field for one), so
// ticket_time is 0. On result 0 the client lists the reply's sorties and
// takes the action points itself (ConsumeStamina); the day's handover time
// isn't touched (no AddCostTime). Non-zero shows chapter_auto_start_fail.
func (game *Game) handleStartWorldChapterAuto(client *connection.Client, req *protobuf.CS_13018) error {
	ctx := context.Background()
	now := serverNow()
	var list []*protobuf.CHAPTER_AUTO_BATTLE
	err := game.tx(ctx, client, func(tx *Tx) error {
		var err error
		list, err = tx.startWorldChapterAuto(req.GetMapIdList(), now)
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_13019{Result: result(err), ChapterAutoBattleList: list})
}

func (tx *Tx) startWorldChapterAuto(ids []uint32, now time.Time) ([]*protobuf.CHAPTER_AUTO_BATTLE, error) {
	if len(ids) == 0 || uint32(len(ids)) > worldAutoMapLimit {
		return nil, errRefused
	}
	battles, err := tx.ListChapterAutoBattles(tx.ctx, tx.commanderID)
	if err != nil {
		return nil, err
	}
	if len(battles) > 0 {
		return nil, errRefused
	}
	state, err := tx.worldState(now)
	if err != nil {
		return nil, err
	}
	if !state.activated() || !state.canDelegate() {
		return nil, errRefused
	}
	transport := state.transportEntrances()
	cost := uint32(0)
	for i, id := range ids {
		if slices.Contains(ids[:i], id) || !state.delegable(id, transport) {
			return nil, errRefused
		}
		cost += worldAutoConfigs[id].OilLimit
	}
	if !state.Stamina.consume(cost) {
		return nil, errRefused
	}
	if err := tx.saveWorldState(state); err != nil {
		return nil, err
	}
	var list []*protobuf.CHAPTER_AUTO_BATTLE
	finish := now.Unix()
	for i, id := range ids {
		seconds := worldAutoConfigs[id].TimeCorrection
		finish += int64(seconds)
		row := db.ChapterAutoBattle{
			CommanderID: tx.commanderID,
			Seq:         uint32(i),
			Type:        chapterAutoWorld,
			ChapterID:   id,
			FinishAt:    finish,
			Seconds:     seconds,
		}
		if err := tx.AddChapterAutoBattle(tx.ctx, db.AddChapterAutoBattleParams(row)); err != nil {
			return nil, err
		}
		list = append(list, chapterAutoBattle(row))
	}
	return list, nil
}

// worldAutoCoins are the Operation Supply Coins a delegated sector gives,
// by corrosion: what belfast's loot gives for a Secured clear (worldloot.go,
// the wiki's ~500 / 950 / 1400 / 1750 at C3-C6), C1 and C2 going down by
// the same steps. The client has no table of a delegated sector's rewards
// (world_auto_statistics.award_display only shows their kinds: the T1-T3
// containers, Emergency Repair Packs, coins).
var worldAutoCoins = [...]uint32{0, 200, 350, 500, 950, 1400, 1750, 1750}

// worldAutoLoot is what a delegated sector of corrosion hazard gives: its
// Secured clear coins, a materials container of its corrosion and a chance
// of an Emergency Repair Pack, as award_display shows.
func worldAutoLoot(tx *Tx, state *worldState, randomID uint32, finish int64) []*protobuf.DROPINFO {
	h := worldHazard(worldRandomMaps[randomID].HazardLevel)
	lo, hi := spread(worldAutoCoins[h])
	rng := worldRand(worldSeed(tx.commanderID, state.Count.Activations, randomID) ^ uint64(finish))
	return rollWorldLoot(rng, []worldLoot{
		{dropTypeWorldItem, worldItemCoins, lo, hi, 0},
		{dropTypeItem, worldContainer(h), 1, 2, 0},
		{dropTypeWorldItem, worldItemRepair, 1, 1, 30},
	})
}

// endWorldChapterAuto is the claim of an Operation Siren handover
// (EndChapterAutoCommand, its WORLD branches; the client asks for the
// whole world first, ReqWorldCheck). The reply carries:
//   - world_ap, the action points of the sectors not finished, which the
//     client gives back (UpdateStamina, PlusStamina);
//   - drop_list, each finished sector's loot (worldAutoLoot) and the items
//     of its zone securing reward not taken yet (the client flags the
//     reward taken without adding them), which the client adds
//     (PlayerConst.addTranDrop: world items into the world inventory);
//   - class_exp, the finished sectors' drop_expbook, Lecture Hall
//     proficiency (NavalAcademyProxy:AddProficiency).
//
// The first finished-count sorties (the list by finish time) are the
// sectors the client then marks delegated (WorldAtlas:AddDelegatedMap): in
// random_map_id_list_by_auto, cost maps (chapter_list, generated here as
// entering them would) and secured (clean_chapter). Their zone securing
// rewards not taken are taken (World:FlagMapPressingAward, flag 0), their
// global buffs added (event_reward_slgbuff, World:AddGlobalBuff), as the
// client does. No seconds, oil or permits: a world handover takes none.
func (tx *Tx) endWorldChapterAuto(battles []db.ChapterAutoBattle, now time.Time) (*protobuf.SC_13015, error) {
	state, err := tx.worldState(now)
	if err != nil {
		return nil, err
	}
	var refund, classExp uint32
	var drops []*protobuf.DROPINFO
	for _, battle := range battles {
		config := worldAutoConfigs[battle.ChapterID]
		if battle.FinishAt > now.Unix() {
			refund += config.OilLimit
			continue
		}
		classExp += config.ExpBook
		drops = append(drops, worldAutoLoot(tx, state, battle.ChapterID, battle.FinishAt)...)
		if !state.activated() || slices.Contains(state.AutoMaps, battle.ChapterID) {
			continue
		}
		state.AutoMaps = append(state.AutoMaps, battle.ChapterID)
		m, err := tx.worldMap(state, battle.ChapterID)
		if err != nil {
			return nil, err
		}
		if m != nil {
			state.enterMap(m)
		}
		if !slices.Contains(state.CleanChapters, battle.ChapterID) {
			state.CleanChapters = append(state.CleanChapters, battle.ChapterID)
		}
		for i := range state.ChapterAwards {
			award := &state.ChapterAwards[i]
			if worldColormasks[award.ID].Chapter != battle.ChapterID || award.Flag != 1 {
				continue
			}
			award.Flag = 0
			reward := worldEventCompletes[award.Award]
			drops = append(drops, configDrops(reward.RewardItems)...)
			if len(reward.RewardBuff) == 2 {
				state.Buffs = addWorldBuff(state.Buffs, reward.RewardBuff[0], int64(reward.RewardBuff[1]))
			}
		}
	}
	state.Stamina.plus(refund)
	reply := &protobuf.SC_13015{
		Result:   result(nil),
		DropList: mergeDrops(drops),
		WorldAp:  proto.Uint32(refund),
		ClassExp: proto.Uint32(classExp),
		Oil:      proto.Uint32(0),
		Seconds:  proto.Uint32(0),
	}
	if err := tx.grantWorldDrops(state, reply.DropList); err != nil {
		return nil, err
	}
	if err := tx.saveWorldState(state); err != nil {
		return nil, err
	}
	if err := tx.DeleteChapterAutoBattles(tx.ctx, tx.commanderID); err != nil {
		return nil, err
	}
	return reply, tx.addFlatClassProficiency(classExp, now)
}
