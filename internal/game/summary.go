package game

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// ActivityConst.ACTIVITY_TYPE_SUMMARY: the anniversary look back on the
// commander's career (PlayerSummaryInfoScene).
const activityTypeSummary = 38

// summary thresholds: the ships counted at level 120 and 125, and at 200
// affinity (Ship.intimacy is in hundredths)
const (
	summaryLevelTop     = 120
	summaryLevelBest    = 125
	summaryIntimacyLove = 20000
	mapTypeScenario     = 1 // Map.SCENARIO, expedition_data_by_map.type
	worldTaskTypeMain   = 0 // world_task_data.type of the main story tasks
)

// handleGetSummaryInfo answers GetSummaryInfoCommand
// (controller/command/player/getsummaryinfocommand.lua) for the running
// ACTIVITY_TYPE_SUMMARY activity with what Summary.New (model/vo/summary.lua)
// shows, from the commander's stored data:
//   - register_date, the commander's creation; first_online (Summary
//     doesn't read it) the same.
//   - guild_name, the guild it is in.
//   - chapter_id, the last main story chapter cleared (chapter_template of a
//     Map.SCENARIO map, progress 100); 0, which the client reads as 101,
//     when none.
//   - marry_number, the ships oathed; first_lady_* the first one oathed
//     (template, the name it was given, the oath time).
//   - medal_number, the medals collected (trophies claimed).
//   - furniture_number, the furniture owned, furniture_worth what it costs
//     in furniture coins (furniture_shop_template.dorm_icon_price).
//   - character_id, the first secretary's template.
//   - world_max_task, the last Operation Siren main story task submitted.
//   - collect_num, the archive groups the collection rate counts
//     (CollectionProxy:getCollectionCount), over config_client[2].
//   - combat, the ships' combat power summed (shipCombatPower, without
//     gear): a guess, Summary shows combat^0.667.
//   - ship counts, at level 120 / 125, at 200 affinity; skin_num the skins
//     owned for good, skin_ship_num the ship groups they are for.
//
// No official capture has it: which data each field counts is guessed
// from the names and the pages showing them.
func (game *Game) handleGetSummaryInfo(client *connection.Client, req *protobuf.CS_26021) error {
	ctx := context.Background()
	now := time.Now()
	config, open, err := game.commanderActivity(ctx, client.CommanderID, req.GetActId(), now)
	if err != nil {
		return err
	}
	if !open || config.Type != activityTypeSummary {
		return client.Send(&protobuf.SC_26022{
			Result: result(errRefused), RegisterDate: proto.Uint32(0), GuildName: proto.String(""), ChapterId: proto.Uint32(0),
			MarryNumber: proto.Uint32(0), MedalNumber: proto.Uint32(0), FurnitureNumber: proto.Uint32(0), FurnitureWorth: proto.Uint32(0),
			CharacterId: proto.Uint32(0), FirstLadyId: proto.Uint32(0), FirstLadyName: proto.String(""), FirstLadyTime: proto.Uint32(0),
			FirstOnline: proto.Uint32(0), WorldMaxTask: proto.Uint32(0), CollectNum: proto.Uint32(0), Combat: proto.Uint32(0),
			ShipNumTotal: proto.Uint32(0), ShipNum_120: proto.Uint32(0), ShipNum_125: proto.Uint32(0), Love200Num: proto.Uint32(0),
			SkinNum: proto.Uint32(0), SkinShipNum: proto.Uint32(0),
		})
	}
	reply, err := commanderSummary(ctx, game.DB.Queries, client.CommanderID, now)
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// commanderSummary is SC_26022 for the commander.
func commanderSummary(ctx context.Context, queries *db.Queries, commanderID uint32, now time.Time) (*protobuf.SC_26022, error) {
	commander, err := queries.GetCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	guildName := ""
	member, err := queries.GetGuildMember(ctx, commanderID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return nil, err
	default:
		guild, err := queries.GetGuild(ctx, member.GuildID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		guildName = guild.Name
	}
	chapters, err := queries.ListChapterProgress(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	chapterID := uint32(0)
	for _, row := range chapters {
		template := chapterTemplates[row.ChapterID]
		if row.Progress >= 100 && template != nil && mapChapters[template.mapID].kind == mapTypeScenario {
			chapterID = max(chapterID, row.ChapterID)
		}
	}
	ships, err := queries.ListShipsByCommander(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	var married, top, best, love uint32
	var firstLady *db.Ship
	power := 0.0
	byID := make(map[uint32]db.Ship, len(ships))
	for i, ship := range ships {
		byID[ship.ID] = ship
		if ship.Propose != 0 {
			married++
			if firstLady == nil || ship.Propose < firstLady.Propose {
				firstLady = &ships[i]
			}
		}
		if ship.Level >= summaryLevelTop {
			top++
		}
		if ship.Level >= summaryLevelBest {
			best++
		}
		if ship.Intimacy >= summaryIntimacyLove {
			love++
		}
		power += shipCombatPower(db.GuildOperationShip{TemplateID: ship.TemplateID, Level: ship.Level})
	}
	claims, err := queries.ListTrophyClaims(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	furniture, err := queries.ListFurniture(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	var furnitureCount, furnitureWorth uint32
	for _, row := range furniture {
		furnitureCount += row.Count
		furnitureWorth += row.Count * furnitureShop[row.FurnitureID].DormIconPrice
	}
	secretaries, err := queries.ListSecretaries(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	character := uint32(0)
	if len(secretaries) > 0 {
		character = byID[secretaries[0].ShipID].TemplateID
	}
	world, err := loadWorldState(ctx, queries, commanderID, now)
	if err != nil {
		return nil, err
	}
	worldTask := uint32(0)
	for _, task := range world.Tasks {
		if config, ok := worldTasks[task.ID]; ok && task.SubmitTime != 0 && config.Type == worldTaskTypeMain {
			worldTask = max(worldTask, task.ID)
		}
	}
	groups, err := queries.ListShipGroups(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	collected := uint32(0)
	for _, group := range groups {
		if rankedGroups[group.GroupID] {
			collected++
		}
	}
	skins, err := queries.ListSkins(ctx, db.ListSkinsParams{CommanderID: commanderID, Now: now.Unix()})
	if err != nil {
		return nil, err
	}
	skinCount := uint32(0)
	skinGroups := map[uint32]bool{}
	for _, skin := range skins {
		if skin.EndTime != 0 {
			continue
		}
		skinCount++
		if group := shipSkins[skin.SkinID].group; group != 0 {
			skinGroups[group] = true
		}
	}
	reply := &protobuf.SC_26022{
		Result:          result(nil),
		RegisterDate:    proto.Uint32(uint32(commander.RegisteredAt)),
		GuildName:       proto.String(guildName),
		ChapterId:       proto.Uint32(chapterID),
		MarryNumber:     proto.Uint32(married),
		MedalNumber:     proto.Uint32(uint32(len(claims))),
		FurnitureNumber: proto.Uint32(furnitureCount),
		FurnitureWorth:  proto.Uint32(furnitureWorth),
		CharacterId:     proto.Uint32(character),
		FirstLadyId:     proto.Uint32(0),
		FirstLadyName:   proto.String(""),
		FirstLadyTime:   proto.Uint32(0),
		FirstOnline:     proto.Uint32(uint32(commander.RegisteredAt)),
		WorldMaxTask:    proto.Uint32(worldTask),
		CollectNum:      proto.Uint32(collected),
		Combat:          proto.Uint32(uint32(power)),
		ShipNumTotal:    proto.Uint32(uint32(len(ships))),
		ShipNum_120:     proto.Uint32(top),
		ShipNum_125:     proto.Uint32(best),
		Love200Num:      proto.Uint32(love),
		SkinNum:         proto.Uint32(skinCount),
		SkinShipNum:     proto.Uint32(uint32(len(skinGroups))),
	}
	if firstLady != nil {
		reply.FirstLadyId = proto.Uint32(firstLady.TemplateID)
		reply.FirstLadyName = proto.String(firstLady.Name)
		reply.FirstLadyTime = proto.Uint32(firstLady.Propose)
	}
	return reply, nil
}
