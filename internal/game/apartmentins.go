package game

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The Private Quarters' phone (Dorm3dInsProxy, Dorm3dChatProxy,
// model/vo/dorm3d/instagram/): per ship group (dorm3d_ins_ship_group_template)
// chat topics (dorm3d_ins_chat_group, Dorm3dTopic), video calls
// (dorm3d_ins_telephone_group, Dorm3dInsPhone) and Juustagram posts
// (dorm3d_ins_template, Instagram3Dorm). Each is locked until its
// dorm3d_ins_unlock row is met: the server counts what the commander does
// with the ship (trigger_type, the APA_EVENT_INFO event types the client
// reports in CS_28023 and the ones the room handlers report themselves), and
// once a row's count reaches trigger_num pushes the content in SC_28025.
// Built from the Lua and the tables alone (only an empty SC_28000 was
// captured); the wiki's Private Quarters page says nothing of these rules.

// dorm3d_ins_unlock types (Dorm3dInsProxy.UNLOCK_TYPE_*), also the stored
// content types.
const (
	apartmentInsChat  = 1
	apartmentInsPhone = 2
	apartmentInsPost  = 3
)

// dorm3d_ins_unlock trigger types the server reports itself (the client
// sends the others in CS_28023).
const (
	apartmentEventFavorLevel = 101 // the new Intimacy level
	apartmentEventVisitDays  = 151 // 1 on the first visit of a server day
	apartmentEventVisits     = 152 // 1 per visit (CS_28036)
	apartmentEventCollect    = 153 // 1 per collection item (CS_28011)
	apartmentEventGift       = 157 // the gifts given (CS_28009)
)

// apartmentEventMiniGameScore is the minigame's new high score
// (Dorm3dRoomScene's GAME_RESULT, only sent when it beats the last one).
const apartmentEventMiniGameScore = 159

// apartmentServerEvents are the event types only the server reports: CS_28023
// can't send them.
var apartmentServerEvents = []uint32{apartmentEventFavorLevel, apartmentEventVisitDays, apartmentEventVisits, apartmentEventCollect, apartmentEventGift}

// apartmentEventAbsolute is whether an event type's value is a level or a
// score, kept at its highest, rather than a count added up. The minigame
// score is one: the client only sends it with a new high score, and the
// wiki's "Private Quarters" page (Anchorage's Minigame: each side has 3 HP,
// a win takes three) makes "Obtain 3 points in Anchorage's minigame" winning
// one game, not points added up over several.
func apartmentEventAbsolute(eventType uint32) bool {
	return eventType == apartmentEventFavorLevel || eventType == apartmentEventMiniGameScore
}

// dorm3d_ins_unlock row
type apartmentInsUnlock struct {
	ID          uint32 `json:"id"`
	Type        uint32 `json:"type"`
	Content     uint32 `json:"content"`
	TriggerType uint32 `json:"trigger_type"`
	TriggerNum  uint32 `json:"trigger_num"`
}

// apartmentInsGroup is the ship_group of a dorm3d_ins_ship_group_template or
// dorm3d_ins_telephone_group row.
type apartmentInsGroup struct {
	ShipGroup uint32 `json:"ship_group"`
}

// dorm3d_ins_chat_group row: a topic, content its words
// (dorm3d_ins_chat_language)
type apartmentChatTopic struct {
	ShipGroup uint32          `json:"ship_group"`
	Content   json.RawMessage `json:"content"`
}

// dorm3d_ins_chat_language row: option is "" or the {reply, text} pairs the
// commander picks from
type apartmentChatWord struct {
	Option json.RawMessage `json:"option"`
}

// replies are the reply ids a word offers, none when it asks nothing.
func (word apartmentChatWord) replies() []uint32 {
	var options [][2]json.RawMessage
	if json.Unmarshal(word.Option, &options) != nil {
		return nil
	}
	ids := make([]uint32, 0, len(options))
	for _, option := range options {
		var id uint32
		if json.Unmarshal(option[0], &id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// dorm3d_ins_template row: a post, discuss the comments the commander can
// make on it (dorm3d_ins_player_template)
type apartmentPost struct {
	ShipGroup uint32          `json:"ship_group"`
	Discuss   json.RawMessage `json:"discuss"`
}

// dorm3d_ins_player_template row: message is one text per pick (the
// CS_28028 value, from 1)
type apartmentComment struct {
	Message []string `json:"message"`
}

var (
	apartmentInsUnlocks     map[uint32]apartmentInsUnlock
	apartmentInsShipGroups  map[uint32]apartmentInsGroup
	apartmentChatTopics     map[uint32]apartmentChatTopic
	apartmentChatWords      map[uint32]apartmentChatWord
	apartmentPhoneCalls     map[uint32]apartmentInsGroup
	apartmentPosts          map[uint32]apartmentPost
	apartmentComments       map[uint32]apartmentComment
	apartmentUnlocksByEvent map[uint32][]apartmentInsUnlock // by trigger_type, in id order
)

func loadApartmentInsData() error {
	if err := gamedata.LoadAll(
		gamedata.Load("dorm3d_ins_unlock", &apartmentInsUnlocks),
		gamedata.Load("dorm3d_ins_ship_group_template", &apartmentInsShipGroups),
		gamedata.Load("dorm3d_ins_chat_group", &apartmentChatTopics),
		gamedata.Load("dorm3d_ins_chat_language", &apartmentChatWords),
		gamedata.Load("dorm3d_ins_telephone_group", &apartmentPhoneCalls),
		gamedata.Load("dorm3d_ins_template", &apartmentPosts),
		gamedata.Load("dorm3d_ins_player_template", &apartmentComments),
	); err != nil {
		return err
	}
	apartmentUnlocksByEvent = map[uint32][]apartmentInsUnlock{}
	for _, row := range apartmentInsUnlocks {
		apartmentUnlocksByEvent[row.TriggerType] = append(apartmentUnlocksByEvent[row.TriggerType], row)
	}
	for _, rows := range apartmentUnlocksByEvent {
		slices.SortFunc(rows, func(a, b apartmentInsUnlock) int { return int(a.ID) - int(b.ID) })
	}
	return nil
}

// apartmentContentGroup is the ship group of a content of a type, from its
// own table row.
func apartmentContentGroup(contentType, id uint32) (uint32, bool) {
	switch contentType {
	case apartmentInsChat:
		row, ok := apartmentChatTopics[id]
		return row.ShipGroup, ok
	case apartmentInsPhone:
		row, ok := apartmentPhoneCalls[id]
		return row.ShipGroup, ok
	case apartmentInsPost:
		row, ok := apartmentPosts[id]
		return row.ShipGroup, ok
	}
	return 0, false
}

// apartmentIns are the APARTMENT_INS of SC_28000 / SC_28002, one per ship
// group in order, empty for a group with nothing unlocked: the client only
// builds a group's posts from one (Dorm3dInsProxy:HandleInsData), and
// UnlockInstagram reads them.
func apartmentIns(ctx context.Context, queries *db.Queries, commanderID uint32, groups []uint32) ([]*protobuf.APARTMENT_INS, error) {
	chats, err := queries.ListApartmentChats(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	contents, err := queries.ListApartmentInsContents(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	replies, err := queries.ListApartmentInsReplies(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	type key struct{ contentType, id uint32 }
	replyLists := map[key][]db.ListApartmentInsRepliesRow{}
	for _, reply := range replies {
		at := key{reply.Type, reply.ContentID}
		replyLists[at] = append(replyLists[at], reply)
	}
	byGroup := make(map[uint32]*protobuf.APARTMENT_INS, len(groups))
	list := make([]*protobuf.APARTMENT_INS, 0, len(groups))
	for _, group := range groups {
		ins := &protobuf.APARTMENT_INS{
			ShipGroup: proto.Uint32(group),
			CareFlag:  proto.Uint32(0),
			CurBack:   proto.Uint32(0),
			CurCommId: proto.Uint32(0),
		}
		byGroup[group] = ins
		list = append(list, ins)
	}
	for _, chat := range chats {
		if ins, ok := byGroup[chat.ShipGroup]; ok {
			ins.CareFlag = proto.Uint32(chat.CareFlag)
			ins.CurBack = proto.Uint32(chat.CurBack)
			ins.CurCommId = proto.Uint32(chat.CurCommID)
		}
	}
	for _, content := range contents {
		ins, ok := byGroup[content.ShipGroup]
		if !ok {
			continue
		}
		replies := replyLists[key{content.Type, content.ContentID}]
		switch content.Type {
		case apartmentInsChat:
			topic := &protobuf.COMM_INFO{Id: proto.Uint32(content.ContentID), Time: proto.Uint32(content.Time), ReadFlag: proto.Uint32(content.ReadFlag)}
			for _, reply := range replies {
				topic.ReplyList = append(topic.ReplyList, &protobuf.KEYVALUE_P28{Key: proto.Uint32(reply.ReplyKey), Value: proto.Uint32(reply.Value)})
			}
			ins.CommList = append(ins.CommList, topic)
		case apartmentInsPhone:
			ins.PhoneList = append(ins.PhoneList, &protobuf.PHONE_INFO{Id: proto.Uint32(content.ContentID), Time: proto.Uint32(content.Time), ReadFlag: proto.Uint32(content.ReadFlag)})
		case apartmentInsPost:
			post := &protobuf.FRIEND_CIRCLE_INFO{
				Id:       proto.Uint32(content.ContentID),
				Time:     proto.Uint32(content.Time),
				ReadFlag: proto.Uint32(content.ReadFlag),
				GoodFlag: proto.Uint32(content.GoodFlag),
				ExitTime: proto.Uint32(content.ExitTime),
			}
			for _, reply := range replies {
				post.ReplyList = append(post.ReplyList, &protobuf.REPLY_FRIEND{Key: proto.Uint32(reply.ReplyKey), Value: proto.Uint32(reply.Value), Time: proto.Uint32(reply.Time)})
			}
			ins.FriendList = append(ins.FriendList, post)
		}
	}
	return list, nil
}

// apartmentEvent records an event of a ship group (a count added up, or the
// highest level / score for the absolute ones) and unlocks, pushing them in
// SC_28025 (Dorm3dInsProxy's handler: a chat topic, a call or a post with
// its unlock time), the contents of that group whose dorm3d_ins_unlock row
// the new value reaches. Events of a shipgirl whose room isn't open are
// not counted.
func (tx *Tx) apartmentEvent(group, eventType, value uint32, now time.Time) error {
	if value == 0 {
		return nil
	}
	// a shipgirl whose room isn't open got no APARTMENT_INS: the client
	// has no Juustagram list to unlock a post in (Dorm3dInsProxy
	// UnlockInstagram), so nothing of hers counts yet
	if _, err := tx.GetApartmentShip(tx.ctx, db.GetApartmentShipParams{CommanderID: tx.commanderID, ShipGroup: group}); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	var total uint32
	var err error
	if apartmentEventAbsolute(eventType) {
		total, err = tx.MaxApartmentEvent(tx.ctx, db.MaxApartmentEventParams{CommanderID: tx.commanderID, ShipGroup: group, EventType: eventType, Value: value})
	} else {
		total, err = tx.AddApartmentEvent(tx.ctx, db.AddApartmentEventParams{CommanderID: tx.commanderID, ShipGroup: group, EventType: eventType, Value: value})
	}
	if err != nil {
		return err
	}
	at := uint32(now.Unix())
	var unlocked []*protobuf.ACT_INFO
	for _, row := range apartmentUnlocksByEvent[eventType] {
		if total < row.TriggerNum {
			continue
		}
		if owner, ok := apartmentContentGroup(row.Type, row.Content); !ok || owner != group {
			continue
		}
		n, err := tx.UnlockApartmentInsContent(tx.ctx, db.UnlockApartmentInsContentParams{CommanderID: tx.commanderID, Type: row.Type, ContentID: row.Content, ShipGroup: group, Time: at})
		if err != nil {
			return err
		}
		if n > 0 {
			unlocked = append(unlocked, &protobuf.ACT_INFO{ShipId: proto.Uint32(group), Type: proto.Uint32(row.Type), ActId: proto.Uint32(row.Content), Time: proto.Uint32(at)})
		}
	}
	if len(unlocked) > 0 {
		tx.push(&protobuf.SC_28025{List: unlocked})
	}
	return nil
}

// apartmentInsContent is an unlocked content of a ship group, errRefused
// when it isn't one.
func (tx *Tx) apartmentInsContent(contentType, id, group uint32) (db.GetApartmentInsContentRow, error) {
	content, err := tx.GetApartmentInsContent(tx.ctx, db.GetApartmentInsContentParams{CommanderID: tx.commanderID, Type: contentType, ContentID: id})
	if errors.Is(err, sql.ErrNoRows) || err == nil && content.ShipGroup != group {
		return content, errRefused
	}
	return content, err
}

// apartmentShipGroup refuses a ship group with no phone.
func apartmentShipGroup(group uint32) error {
	if _, ok := apartmentInsShipGroups[group]; !ok {
		return errRefused
	}
	return nil
}

// handleApartmentEvents answers Dorm3dChatProxy:TriggerEvent (CS_28023, the
// room's talks, touches, gifts, minigames, photos... with the time of day:
// 110-114 by day, 115-119 at night, 155-161 at any time) with SC_28024:
// each event counts for its ship group's unlocks. The event types the
// server reports itself are refused.
func (game *Game) handleApartmentEvents(client *connection.Client, req *protobuf.CS_28023) error {
	ctx := context.Background()
	now := time.Now()
	err := game.tx(ctx, client, func(tx *Tx) error {
		for _, event := range req.GetEventList() {
			if err := apartmentShipGroup(event.GetShipId()); err != nil {
				return err
			}
			if slices.Contains(apartmentServerEvents, event.GetEventType()) {
				return errRefused
			}
			if err := tx.apartmentEvent(event.GetShipId(), event.GetEventType(), event.GetValue(), now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_28024{Result: result(err)})
}

// handleApartmentInsMark answers the marks on unlocked contents of a ship
// group (CS_28026) with SC_28027: type 1 the chat topics read
// (Dorm3dChatCommand's APARTMENT_CHAT_SET_READTIP), 2 a call watched
// (Dorm3dInsPhoneOpCommand), and on a post (ApartmentInstagramOpCommand,
// Instagram3Dorm.OP_*) 3 read, 4 liked, 5 shared (nothing to keep) and 6
// left at the time it closed (SetExitTime).
func (game *Game) handleApartmentInsMark(client *connection.Client, req *protobuf.CS_28026) error {
	ctx := context.Background()
	now := time.Now()
	err := apartmentShipGroup(req.GetShipId())
	if err == nil {
		err = game.tx(ctx, client, func(tx *Tx) error {
			contentType := uint32(apartmentInsPost)
			switch req.GetType() {
			case 1:
				contentType = apartmentInsChat
			case 2:
				contentType = apartmentInsPhone
			case 3, 4, 5, 6:
			default:
				return errRefused
			}
			for _, id := range req.GetIdList() {
				if _, err := tx.apartmentInsContent(contentType, id, req.GetShipId()); err != nil {
					return err
				}
				at := db.SetApartmentInsReadParams{ReadFlag: 1, CommanderID: client.CommanderID, Type: contentType, ContentID: id}
				var err error
				switch req.GetType() {
				case 1, 2, 3:
					err = tx.SetApartmentInsRead(ctx, at)
				case 4:
					err = tx.SetApartmentInsGood(ctx, db.SetApartmentInsGoodParams{CommanderID: at.CommanderID, Type: at.Type, ContentID: at.ContentID})
				case 6:
					err = tx.SetApartmentInsExit(ctx, db.SetApartmentInsExitParams{ExitTime: uint32(now.Unix()), CommanderID: at.CommanderID, Type: at.Type, ContentID: at.ContentID})
				}
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_28027{Result: result(err)})
}

// handleApartmentInsReply answers a reply (CS_28028) with SC_28029: type 1
// to a word of an unlocked chat topic (Dorm3dChatCommand's
// APARTMENT_CHAT_REPLY: chat_id the word, one of the topic's, value one of
// its options), which leaves the topic unread as the client does
// (SetTopicReaded(topic, 0)); type 2 (Instagram3Dorm.OP_DISCUSS) a comment
// on an unlocked post (chat_id one of its discuss, value the pick, from 1,
// among its messages). A word or a comment is answered once
// (Instagram3Dorm:MarkReply drops it from the replyable ones). drop_list
// stays empty: the EN client has no red packets (dorm3d_ins_redpackage).
func (game *Game) handleApartmentInsReply(client *connection.Client, req *protobuf.CS_28028) error {
	ctx := context.Background()
	now := time.Now()
	err := apartmentShipGroup(req.GetShipId())
	if err == nil {
		err = game.tx(ctx, client, func(tx *Tx) error {
			contentType := uint32(apartmentInsChat)
			switch req.GetType() {
			case 1:
				if _, err := tx.apartmentInsContent(apartmentInsChat, req.GetId(), req.GetShipId()); err != nil {
					return err
				}
				if !slices.Contains(gamedata.IDs(apartmentChatTopics[req.GetId()].Content), req.GetChatId()) {
					return errRefused
				}
				// GUESS: a word without options takes any value (the red
				// packets' ids, which EN has none of)
				if options := apartmentChatWords[req.GetChatId()].replies(); len(options) > 0 && !slices.Contains(options, req.GetValue()) {
					return errRefused
				}
			case 2:
				contentType = apartmentInsPost
				if _, err := tx.apartmentInsContent(apartmentInsPost, req.GetId(), req.GetShipId()); err != nil {
					return err
				}
				comment, ok := apartmentComments[req.GetChatId()]
				if !ok || !slices.Contains(gamedata.IDs(apartmentPosts[req.GetId()].Discuss), req.GetChatId()) || req.GetValue() == 0 || int(req.GetValue()) > len(comment.Message) {
					return errRefused
				}
			default:
				return errRefused
			}
			n, err := tx.AddApartmentInsReply(ctx, db.AddApartmentInsReplyParams{CommanderID: client.CommanderID, Type: contentType, ContentID: req.GetId(), ReplyKey: req.GetChatId(), Value: req.GetValue(), Time: uint32(now.Unix())})
			if err != nil {
				return err
			}
			if n == 0 {
				return errRefused
			}
			if contentType == apartmentInsChat {
				return tx.SetApartmentInsRead(ctx, db.SetApartmentInsReadParams{ReadFlag: 0, CommanderID: client.CommanderID, Type: apartmentInsChat, ContentID: req.GetId()})
			}
			return nil
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_28029{Result: result(err)})
}

// handleApartmentChatBackground answers Dorm3dChatCommand's
// APARTMENT_CHAT_SET_SKIN (CS_28030, the chat's background: back_id a skin
// of the ship, 0 the default) with SC_28031. GUESS: any back_id is taken,
// the client only offers the skins it shows (Dorm3dChat:SetBackgrounds).
func (game *Game) handleApartmentChatBackground(client *connection.Client, req *protobuf.CS_28030) error {
	err := apartmentShipGroup(req.GetShipId())
	if err == nil {
		err = game.DB.Queries.SetApartmentChatBack(context.Background(), db.SetApartmentChatBackParams{CommanderID: client.CommanderID, ShipGroup: req.GetShipId(), CurBack: req.GetBackId()})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_28031{Result: result(err)})
}

// handleApartmentChatCare answers Dorm3dChatCommand's
// APARTMENT_CHAT_SET_CARE (CS_28032, the chat's care toggle, value 0 or 1)
// with SC_28033.
func (game *Game) handleApartmentChatCare(client *connection.Client, req *protobuf.CS_28032) error {
	err := apartmentShipGroup(req.GetShipId())
	if err == nil && req.GetValue() > 1 {
		err = errRefused
	}
	if err == nil {
		err = game.DB.Queries.SetApartmentChatCare(context.Background(), db.SetApartmentChatCareParams{CommanderID: client.CommanderID, ShipGroup: req.GetShipId(), CareFlag: req.GetValue()})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_28033{Result: result(err)})
}

// handleApartmentChatTopic answers Dorm3dChatCommand's
// APARTMENT_CHAT_SET_TOPIC (CS_28034, the topic the chat shows: one the
// ship unlocked) with SC_28035.
func (game *Game) handleApartmentChatTopic(client *connection.Client, req *protobuf.CS_28034) error {
	ctx := context.Background()
	err := apartmentShipGroup(req.GetShipId())
	if err == nil {
		err = game.tx(ctx, client, func(tx *Tx) error {
			if _, err := tx.apartmentInsContent(apartmentInsChat, req.GetCommId(), req.GetShipId()); err != nil {
				return err
			}
			return tx.SetApartmentChatTopic(ctx, db.SetApartmentChatTopicParams{CommanderID: client.CommanderID, ShipGroup: req.GetShipId(), CurCommID: req.GetCommId()})
		})
	}
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_28035{Result: result(err)})
}
