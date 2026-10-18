package game

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The cattery (CommanderHome, model/vo/commander/cattery/): slots
// (Comf-Forts, nest_number of them open at the cattery's level) a cat lives
// in, earning exp_number EXP an hour, and three daily ops on the housed
// cats, each once a day per slot and per cat (bwiki 指挥喵: 每个喵窝每日只能进
// 行一轮互动，每只指挥喵每日只能被进行一轮互动).

// Cattery ops (CS_25028 type, Cattery.OP_* bits in op_flag).
const (
	catteryClean = 1
	catteryFeed  = 2
	catteryPlay  = 3
)

// catteryOpBits are a slot's op_flag bits by op (Cattery.OP_CLEAR, OP_FEED,
// OP_PLAY).
var catteryOpBits = map[uint32]uint32{catteryClean: 1, catteryFeed: 2, catteryPlay: 4}

// catterySlot is a slot's stored state, an empty one with every op
// available when it has none.
func catterySlot(slots []db.MeowfficerCattery, commanderID, id uint32) db.MeowfficerCattery {
	if i := slices.IndexFunc(slots, func(slot db.MeowfficerCattery) bool { return slot.SlotID == id }); i >= 0 {
		return slots[i]
	}
	return db.MeowfficerCattery{CommanderID: commanderID, SlotID: id, Style: 1}
}

// catteryOpTime is when a slot's op was last done.
func catteryOpTime(slot db.MeowfficerCattery, op uint32) int64 {
	switch op {
	case catteryClean:
		return slot.CleanTime
	case catteryFeed:
		return slot.FeedTime
	}
	return slot.PlayTime
}

// meowfficerOpTime is when a cat last took part in an op (Commander
// ExistCleanFlag...).
func meowfficerOpTime(cat db.Meowfficer, op uint32) int64 {
	switch op {
	case catteryClean:
		return cat.HomeCleanTime
	case catteryFeed:
		return cat.HomeFeedTime
	}
	return cat.HomePlayTime
}

// catteryOpFlag is a slot's op_flag: the ops not done today.
func catteryOpFlag(slot db.MeowfficerCattery, now time.Time) uint32 {
	flag := uint32(0)
	for op, bit := range catteryOpBits {
		if !sameServerDay(time.Unix(catteryOpTime(slot, op), 0), now) {
			flag |= bit
		}
	}
	return flag
}

// addMeowfficerExp is a cat's level and EXP after gaining exp, as
// Commander:addExp: nothing past the last level.
func addMeowfficerExp(cat db.Meowfficer, exp uint32) (level, rest uint32) {
	top := meowfficerMaxLevel()
	level, rest = cat.Level, cat.Exp
	if level >= top {
		return level, rest
	}
	rarity := meowfficerTemplates[cat.TemplateID].Rarity
	rest += exp
	for level < top {
		need := meowfficerLevelExp(level, rarity)
		if need == 0 || rest < need {
			break
		}
		rest -= need
		level++
	}
	return level, rest
}

// settleCattery gives the cat housed in slot the EXP it earned since its
// last settlement (exp_number an hour, CalcCatteryExpCommand), from now
// on counting from now; it returns the cat as it is now and the EXP given.
func settleCattery(tx *Tx, home catteryLevel, slot *db.MeowfficerCattery, now int64) (db.Meowfficer, uint32, error) {
	cat, err := tx.GetMeowfficer(tx.ctx, db.GetMeowfficerParams{CommanderID: tx.commanderID, ID: slot.MeowfficerID})
	if err != nil {
		return cat, 0, err
	}
	gained := uint32(0)
	if now > slot.ExpTime {
		gained = uint32(int64(home.ExpNumber) * (now - slot.ExpTime) / 3600)
	}
	slot.ExpTime = now
	if gained == 0 {
		return cat, 0, nil
	}
	cat.Level, cat.Exp = addMeowfficerExp(cat, gained)
	err = tx.SetMeowfficerLevel(tx.ctx, db.SetMeowfficerLevelParams{Level: cat.Level, Exp: cat.Exp, CommanderID: tx.commanderID, ID: cat.ID})
	return cat, gained, err
}

func saveCatterySlot(tx *Tx, slot db.MeowfficerCattery) error {
	return tx.SaveMeowfficerCattery(tx.ctx, db.SaveMeowfficerCatteryParams{
		CommanderID: tx.commanderID, SlotID: slot.SlotID, MeowfficerID: slot.MeowfficerID, Style: slot.Style,
		ExpTime: slot.ExpTime, CleanTime: slot.CleanTime, FeedTime: slot.FeedTime, PlayTime: slot.PlayTime,
	})
}

// catteryCleanLoss is the cleanliness the cattery loses a day,
// commander_home flower[2].
func catteryCleanLoss(home catteryLevel) uint32 {
	var flower []json.RawMessage
	var loss uint32
	if json.Unmarshal(home.Flower, &flower) == nil && len(flower) > 1 {
		json.Unmarshal(flower[1], &loss)
	}
	return loss
}

// settleCatteryClean takes the cleanliness lost since it was last settled
// off: flower[2] each server day (CommanderProxy's day call, ReduceClean
// once any slot is dirty, which every housed cat is after ResetCatteryOP and
// every slot the client fills in itself), down to 0. The first settlement
// only starts the count.
func settleCatteryClean(tx *Tx, state *db.MeowfficerState, now time.Time) error {
	if state.CleanTime != 0 {
		loss := catteryCleanLoss(cattery(state.HomeLevel))
		for day := nextDay(time.Unix(state.CleanTime, 0)); !day.After(now) && loss > 0 && state.HomeClean > 0; day = day.AddDate(0, 0, 1) {
			state.HomeClean -= min(loss, state.HomeClean)
		}
	}
	state.CleanTime = now.Unix()
	if err := tx.CreateMeowfficerState(tx.ctx, tx.commanderID); err != nil {
		return err
	}
	return tx.SetMeowfficerHomeClean(tx.ctx, db.SetMeowfficerHomeCleanParams{
		HomeClean: state.HomeClean, CleanTime: state.CleanTime, CommanderID: tx.commanderID,
	})
}

// handleCommanderHome answers GetCommanderHomeCommand (CS_25026, sent once
// SC_25001 is in): the cattery's level, EXP and cleanliness and its open
// slots. A housed cat's EXP is settled first and sent with its slot
// (commander_level, commander_exp, cache_exp the EXP it just got), the
// client counting from exp_time on. A new account's is level 1 with its one
// slot empty, every op available (captured).
func (game *Game) handleCommanderHome(client *connection.Client, req *protobuf.CS_25026) error {
	now := time.Now()
	reply := &protobuf.SC_25027{}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := meowfficerState(tx.ctx, tx.Queries, tx.commanderID)
		if err != nil {
			return err
		}
		slots, err := tx.ListMeowfficerCatteries(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		if err := settleCatteryClean(tx, &state, now); err != nil {
			return err
		}
		home := cattery(state.HomeLevel)
		reply.Level = proto.Uint32(state.HomeLevel)
		reply.Exp = proto.Uint32(state.HomeExp)
		reply.Clean = proto.Uint32(state.HomeClean)
		for id := uint32(1); id <= max(home.NestNumber, 1) && id <= catterySlotCount; id++ {
			slot := catterySlot(slots, tx.commanderID, id)
			info := &protobuf.COMMANDERHOMESLOT{Id: proto.Uint32(id), Style: proto.Uint32(slot.Style)}
			if slot.MeowfficerID != 0 {
				cat, gained, err := settleCattery(tx, home, &slot, now.Unix())
				if err != nil {
					return err
				}
				if err := saveCatterySlot(tx, slot); err != nil {
					return err
				}
				info.CommanderId = proto.Uint32(cat.ID)
				info.CommanderLevel = proto.Uint32(cat.Level)
				info.CommanderExp = proto.Uint32(cat.Exp)
				info.CacheExp = proto.Uint32(gained)
			}
			info.OpFlag = proto.Uint32(catteryOpFlag(slot, now))
			info.ExpTime = proto.Uint32(uint32(slot.ExpTime))
			reply.Slots = append(reply.Slots, info)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// handleCatteryScene takes OpenOrCloseCatteryCommand (CS_25036, entering
// and leaving the meowfficer scene, is_open 0 on entering): the client only
// stops showing cached EXP; there is no reply.
//
//belfast:static
func (game *Game) handleCatteryScene(client *connection.Client, req *protobuf.CS_25036) error {
	return nil
}

// handlePutMeowfficerInCattery houses a cat in a slot, or takes it out with
// commander_id 0 (PutCommanderInCatteryCommand, CS_25030, NG0029). The slot
// has to be open; a cat already housed elsewhere is refused. The cat that
// leaves (taken out or replaced) has its EXP settled and sent back
// (commander_level, commander_exp; level 0 when none leaves); time is when
// the new cat's EXP counts from. Built from the Lua alone.
func (game *Game) handlePutMeowfficerInCattery(client *connection.Client, req *protobuf.CS_25030) error {
	now := time.Now().Unix()
	slotID, catID := req.GetSlotidx(), req.GetCommanderId()
	reply := &protobuf.SC_25031{Time: proto.Uint32(uint32(now))}
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := meowfficerState(tx.ctx, tx.Queries, tx.commanderID)
		if err != nil {
			return err
		}
		home := cattery(state.HomeLevel)
		if slotID < 1 || slotID > home.NestNumber || slotID > catterySlotCount {
			return errRefused
		}
		slots, err := tx.ListMeowfficerCatteries(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		slot := catterySlot(slots, tx.commanderID, slotID)
		if catID == 0 && slot.MeowfficerID == 0 || catID != 0 && slot.MeowfficerID == catID {
			return errRefused
		}
		if catID != 0 {
			if _, err := tx.GetMeowfficer(tx.ctx, db.GetMeowfficerParams{CommanderID: tx.commanderID, ID: catID}); err != nil {
				return errRefused
			}
			if slices.ContainsFunc(slots, func(other db.MeowfficerCattery) bool { return other.MeowfficerID == catID }) {
				return errRefused
			}
		}
		if slot.MeowfficerID != 0 {
			cat, _, err := settleCattery(tx, home, &slot, now)
			if err != nil {
				return err
			}
			reply.CommanderLevel = proto.Uint32(cat.Level)
			reply.CommanderExp = proto.Uint32(cat.Exp)
		}
		slot.MeowfficerID = catID
		slot.ExpTime = now
		return saveCatterySlot(tx, slot)
	})
	if err != nil && !refused(err) {
		return err
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// handleCatteryOp cleans, feeds or plays with the housed cats
// (CommanderCatteryOPCommand, CS_25028 type 1, 2, 3; NG0029 does all
// three): every open slot whose op is still available today and whose cat
// hasn't had it today takes part. Each gives the cattery EXP (clean_exp,
// feed_level[3], teast_level[3] per cat, levelling it up with home_exp); a
// clean also adds flower[1] cleanliness, a feed feed_level[2] EXP to each
// cat (the client adds it itself), a play the awards of teast_level[2] per
// cat (catteryPlayAwards). Replies the cattery's level and EXP, the awards
// and op_time, the cats' new op time. Built from the Lua and the bwiki.
func (game *Game) handleCatteryOp(client *connection.Client, req *protobuf.CS_25028) error {
	now := time.Now()
	op := req.GetType()
	reply := &protobuf.SC_25029{OpTime: proto.Uint32(uint32(now.Unix()))}
	err := errRefused
	if _, ok := catteryOpBits[op]; ok {
		err = game.tx(context.Background(), client, func(tx *Tx) error {
			state, err := meowfficerState(tx.ctx, tx.Queries, tx.commanderID)
			if err != nil {
				return err
			}
			home := cattery(state.HomeLevel)
			slots, err := tx.ListMeowfficerCatteries(tx.ctx, tx.commanderID)
			if err != nil {
				return err
			}
			cats := uint32(0)
			for _, slot := range slots {
				if slot.MeowfficerID == 0 || slot.SlotID > home.NestNumber || sameServerDay(time.Unix(catteryOpTime(slot, op), 0), now) {
					continue
				}
				cat, err := tx.GetMeowfficer(tx.ctx, db.GetMeowfficerParams{CommanderID: tx.commanderID, ID: slot.MeowfficerID})
				if err != nil {
					return err
				}
				if sameServerDay(time.Unix(meowfficerOpTime(cat, op), 0), now) {
					continue
				}
				if err := doCatteryOp(tx, home, op, &slot, cat, now.Unix()); err != nil {
					return err
				}
				cats++
			}
			if cats == 0 {
				return errRefused
			}
			if err := settleCatteryClean(tx, &state, now); err != nil {
				return err
			}
			homeExp := home.CleanExp
			switch op {
			case catteryClean:
				if flower := gamedata.IDs(home.Flower); len(flower) > 0 {
					state.HomeClean += flower[0]
				}
			case catteryFeed:
				homeExp = nth(home.FeedLevel, 3)
			case catteryPlay:
				homeExp = nth(home.TeastLevel, 3)
				for range cats {
					for _, award := range catteryPlayAwards[nth(home.TeastLevel, 2)] {
						if rand.Uint32N(100) < award.chance {
							reply.Awards = mergeDrop(reply.Awards, award.kind, award.id, award.count)
						}
					}
				}
				if err := tx.grantDrops(reply.Awards); err != nil {
					return err
				}
			}
			state.HomeExp += homeExp * cats
			for state.HomeLevel < catteryMaxLevel() {
				need := cattery(state.HomeLevel).HomeExp
				if need == 0 || state.HomeExp < need {
					break
				}
				state.HomeExp -= need
				state.HomeLevel++
			}
			reply.Level = proto.Uint32(state.HomeLevel)
			reply.Exp = proto.Uint32(state.HomeExp)
			if err := tx.CreateMeowfficerState(tx.ctx, tx.commanderID); err != nil {
				return err
			}
			return tx.SetMeowfficerHome(tx.ctx, db.SetMeowfficerHomeParams{
				HomeLevel: state.HomeLevel, HomeExp: state.HomeExp, HomeClean: state.HomeClean, CommanderID: tx.commanderID,
			})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	if err != nil {
		reply.Awards = nil
	}
	reply.Result = result(err)
	return client.Send(reply)
}

// doCatteryOp records an op done on a slot and its cat; a feed gives the
// cat feed_level[2] EXP.
func doCatteryOp(tx *Tx, home catteryLevel, op uint32, slot *db.MeowfficerCattery, cat db.Meowfficer, now int64) error {
	var err error
	switch op {
	case catteryClean:
		slot.CleanTime = now
		err = tx.SetMeowfficerHomeCleanTime(tx.ctx, db.SetMeowfficerHomeCleanTimeParams{HomeCleanTime: now, CommanderID: tx.commanderID, ID: cat.ID})
	case catteryFeed:
		slot.FeedTime = now
		err = tx.SetMeowfficerHomeFeedTime(tx.ctx, db.SetMeowfficerHomeFeedTimeParams{HomeFeedTime: now, CommanderID: tx.commanderID, ID: cat.ID})
		if err == nil {
			level, exp := addMeowfficerExp(cat, nth(home.FeedLevel, 2))
			err = tx.SetMeowfficerLevel(tx.ctx, db.SetMeowfficerLevelParams{Level: level, Exp: exp, CommanderID: tx.commanderID, ID: cat.ID})
		}
	case catteryPlay:
		slot.PlayTime = now
		err = tx.SetMeowfficerHomePlayTime(tx.ctx, db.SetMeowfficerHomePlayTimeParams{HomePlayTime: now, CommanderID: tx.commanderID, ID: cat.ID})
	}
	if err != nil {
		return err
	}
	return saveCatterySlot(tx, *slot)
}

// handleChangeCatteryStyle changes an open slot's style
// (CommanderChangeCatteryStyleCommand, CS_25032): one of the cattery
// level's nest_appearance (the others show locked, cathome_style_unlock),
// not the one it has. Built from the Lua alone.
func (game *Game) handleChangeCatteryStyle(client *connection.Client, req *protobuf.CS_25032) error {
	slotID, style := req.GetSlotidx(), req.GetStyleidx()
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		state, err := meowfficerState(tx.ctx, tx.Queries, tx.commanderID)
		if err != nil {
			return err
		}
		home := cattery(state.HomeLevel)
		if slotID < 1 || slotID > home.NestNumber || slotID > catterySlotCount || !slices.Contains(home.NestStyles, style) {
			return errRefused
		}
		slots, err := tx.ListMeowfficerCatteries(tx.ctx, tx.commanderID)
		if err != nil {
			return err
		}
		slot := catterySlot(slots, tx.commanderID, slotID)
		if slot.Style == style {
			return errRefused
		}
		slot.Style = style
		return saveCatterySlot(tx, slot)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_25033{Result: result(err)})
}
