package game

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/packets"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// juusPosts are the Juus posts the client can't build from its tables
// (activity_ins_template is_active 1: InitByServer), as first pushed by the
// official server: an SC_11706 in text format, server/juus_posts.txtpb of the
// data dir. Without it there are no posts.
var juusPosts map[uint32]*protobuf.INS_MESSAGE

func loadJuusPosts() error {
	juusPosts = map[uint32]*protobuf.INS_MESSAGE{}
	text, err := os.ReadFile(filepath.Join(regionDataDir(), "server", "juus_posts.txtpb"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var list protobuf.SC_11706
	if err := prototext.Unmarshal(text, &list); err != nil {
		return fmt.Errorf("juus posts: %w", err)
	}
	for _, post := range list.InsMessageList {
		juusPosts[post.GetId()] = post
	}
	return nil
}

// juusReply is an NPC answer to the commander's comment, posted delay
// seconds after it.
type juusReply struct {
	id    uint32
	delay uint32
}

// juusOptionReplies are the NPC answers to a comment option (post id ->
// 1-based option -> answers), server-side data; only the options picked in
// the capture are known, the others get no answer.
var juusOptionReplies = map[uint32]map[uint32][]juusReply{
	673: {1: {{7525, 9}}},
	20015: {2: {
		{100509, 56}, {100510, 76}, {100511, 96}, {100512, 116},
		{100513, 136}, {100514, 156}, {100515, 176}, {100516, 196},
	}},
}

// CS_11701 cmd values (ActivityConst.INSTAGRAM_OP_*)
const (
	juusOpUpdate   = 1 // also sent to activate a post once its time came
	juusOpLike     = 3
	juusOpShare    = 4
	juusOpMarkRead = 5
)

// activity_ins_chat_group row: a chat topic and the character it is with
type juusTopicConfig struct {
	ID        uint32 `json:"id"`
	ShipGroup uint32 `json:"ship_group"`
}

// activity_ins_chat_language row: a chat line; type 4 is a red packet whose
// param is an activity_ins_redpackage id
type juusWordConfig struct {
	ID    uint32 `json:"id"`
	Type  uint32 `json:"type"`
	Param any    `json:"param"`
}

const juusWordRedPacket = 4

// activity_ins_redpackage row: content is the drop
type juusRedPacketConfig struct {
	ID      uint32   `json:"id"`
	Content []uint32 `json:"content"`
}

var (
	juusTopicConfigs     map[uint32]juusTopicConfig
	juusWordConfigs      map[uint32]juusWordConfig
	juusRedPacketConfigs map[uint32]juusRedPacketConfig
)

// loadJuusData reads the Juus chats and the posts the server sends.
func loadJuusData() error {
	for _, load := range []func() error{
		loadTable("activity_ins_chat_group", &juusTopicConfigs),
		loadTable("activity_ins_chat_language", &juusWordConfigs),
		loadTable("activity_ins_redpackage", &juusRedPacketConfigs),
		loadJuusPosts,
	} {
		if err := load(); err != nil {
			return err
		}
	}
	return nil
}

func (game *Game) registerJuus() {
	packets.Handle(game.handleJuusPostOp)
	packets.Handle(game.handleJuusComment)
	packets.Handle(game.handleJuusPosts)
	packets.Handle(game.handleJuusChatReply)
	packets.Handle(game.handleJuusTopicsRead)
	packets.Handle(game.handleJuusActivateTopics)
}

// juusMessage is a post as the commander sees it: liked, read, and their
// comment with the answers it got.
func juusMessage(row db.CommanderJuusPost) *protobuf.INS_MESSAGE {
	base, ok := juusPosts[row.PostID]
	if !ok {
		return nil
	}
	post := proto.Clone(base).(*protobuf.INS_MESSAGE)
	if row.IsGood {
		post.IsGood = proto.Uint32(1)
		post.Good = proto.Uint32(post.GetGood() + 1)
	}
	if row.IsRead {
		post.IsRead = proto.Uint32(1)
	}
	if row.CommentDiscuss != 0 {
		for _, discuss := range post.PlayerDiscuss {
			index := int(row.CommentIndex) - 1
			if discuss.GetId() != row.CommentDiscuss || index < 0 || index >= len(discuss.TextList) {
				continue
			}
			discuss.Text = proto.String(discuss.TextList[index])
			discuss.TextList = nil
			discuss.Time = proto.Uint32(row.CommentTime)
			for _, reply := range juusOptionReplies[row.PostID][row.CommentIndex] {
				discuss.NpcReply = append(discuss.NpcReply, reply.id)
				post.NpcReply = append(post.NpcReply, &protobuf.INS_NPC{
					Id:   proto.Uint32(reply.id),
					Time: proto.Uint32(row.CommentTime + reply.delay),
					Text: proto.String(""), // the capture sends none
				})
			}
		}
	}
	return post
}

// handleJuusPosts answers the client asking for posts by id
// (RequestNew/OldInstagramDataCommand) with the ones pushed to the commander;
// a new commander has none (every SC_11706 of the capture is empty).
func (game *Game) handleJuusPosts(client *connection.Client, req *protobuf.CS_11705) error {
	rows, err := game.DB.ListJuusPosts(context.Background(), client.CommanderID)
	if err != nil {
		return err
	}
	byID := make(map[uint32]db.CommanderJuusPost, len(rows))
	for _, row := range rows {
		byID[row.PostID] = row
	}
	reply := &protobuf.SC_11706{}
	for _, id := range req.GetIdList() {
		if row, ok := byID[id]; ok {
			if post := juusMessage(row); post != nil {
				reply.InsMessageList = append(reply.InsMessageList, post)
			}
		}
	}
	return client.Send(reply)
}

// handleJuusPostOp answers InstagramActivityCommand on a post: cmd 1
// activates it (the client's timer sends it once the post's time came) or
// refreshes it, 3 likes, 4 shares, 5 marks it read. The reply is the post.
func (game *Game) handleJuusPostOp(client *connection.Client, req *protobuf.CS_11701) error {
	ctx := context.Background()
	key := db.GetJuusPostParams{CommanderID: client.CommanderID, PostID: req.GetId()}
	if req.GetCmd() == juusOpUpdate {
		if juusPosts[req.GetId()] == nil {
			return client.Send(&protobuf.SC_11702{Result: result(errRefused)})
		}
		if err := game.DB.CreateJuusPost(ctx, db.CreateJuusPostParams{CommanderID: client.CommanderID, PostID: req.GetId()}); err != nil {
			return err
		}
	}
	if _, err := game.DB.GetJuusPost(ctx, key); errors.Is(err, sql.ErrNoRows) {
		return client.Send(&protobuf.SC_11702{Result: result(errRefused)})
	} else if err != nil {
		return err
	}
	switch req.GetCmd() {
	case juusOpLike:
		if err := game.DB.LikeJuusPost(ctx, db.LikeJuusPostParams{CommanderID: client.CommanderID, PostID: req.GetId()}); err != nil {
			return err
		}
	case juusOpMarkRead:
		if err := game.DB.ReadJuusPost(ctx, db.ReadJuusPostParams{CommanderID: client.CommanderID, PostID: req.GetId()}); err != nil {
			return err
		}
	}
	row, err := game.DB.GetJuusPost(ctx, key)
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11702{Result: result(nil), Data: juusMessage(row)})
}

// handleJuusComment posts the comment option the commander picked (discuss is
// the post's player_discuss id, index the 1-based option), the reply is the
// post with it and the NPC answers.
func (game *Game) handleJuusComment(client *connection.Client, req *protobuf.CS_11703) error {
	ctx := context.Background()
	key := db.GetJuusPostParams{CommanderID: client.CommanderID, PostID: req.GetId()}
	row, err := game.DB.GetJuusPost(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return client.Send(&protobuf.SC_11704{Result: result(errRefused)})
	} else if err != nil {
		return err
	}
	valid := false
	if post := juusMessage(row); post != nil && row.CommentDiscuss == 0 {
		for _, discuss := range post.PlayerDiscuss {
			if discuss.GetId() == req.GetDiscuss() && req.GetIndex() >= 1 && int(req.GetIndex()) <= len(discuss.TextList) {
				valid = true
			}
		}
	}
	if !valid {
		return client.Send(&protobuf.SC_11704{Result: result(errRefused)})
	}
	if err := game.DB.CommentJuusPost(ctx, db.CommentJuusPostParams{
		CommentDiscuss: req.GetDiscuss(),
		CommentIndex:   req.GetIndex(),
		CommentTime:    uint32(time.Now().Unix()),
		CommanderID:    client.CommanderID,
		PostID:         req.GetId(),
	}); err != nil {
		return err
	}
	if row, err = game.DB.GetJuusPost(ctx, key); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11704{Result: result(nil), Data: juusMessage(row)})
}

// juusGroups is SC_11711, the Juus chats (InstagramChatProxy): the unlocked
// topics grouped by character, each with the commander's answers. The
// current topic is the latest one unlocked. CS_11710's handler should reply
// with it.
func (game *Game) juusGroups(ctx context.Context, commanderID uint32) (*protobuf.SC_11711, error) {
	topics, err := game.DB.ListJuusTopics(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	replies, err := game.DB.ListJuusReplies(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_11711{}
	groups := map[uint32]*protobuf.JUUS_GROUP{}
	for _, topic := range topics {
		group, ok := groups[topic.ShipGroup]
		if !ok {
			group = &protobuf.JUUS_GROUP{Id: proto.Uint32(topic.ShipGroup)}
			groups[topic.ShipGroup] = group
			msg.Groups = append(msg.Groups, group)
		}
		group.CurChatGroup = proto.Uint32(topic.TopicID)
		readFlag := uint32(0)
		if topic.ReadFlag {
			readFlag = 1
		}
		chat := &protobuf.JUUS_CHAT_GROUP{Id: proto.Uint32(topic.TopicID), OpTime: proto.Uint32(topic.OpTime), ReadFlag: proto.Uint32(readFlag)}
		for _, reply := range replies {
			if reply.TopicID == topic.TopicID {
				chat.ReplyList = append(chat.ReplyList, &protobuf.KEYVALUE_P11{Key: proto.Uint32(reply.ChatID), Value: proto.Uint32(reply.Value)})
			}
		}
		group.ChatGroupList = append(group.ChatGroupList, chat)
	}
	return msg, nil
}

// handleJuusActivateTopics unlocks chat topics (InstagramChatCommand
// ACTIVATE_TOPIC): one result per topic, 0 when it exists, and the unlock
// time.
func (game *Game) handleJuusActivateTopics(client *connection.Client, req *protobuf.CS_11722) error {
	ctx := context.Background()
	now := uint32(time.Now().Unix())
	reply := &protobuf.SC_11723{OpTime: proto.Uint32(now)}
	for _, id := range req.GetChatGroupIdList() {
		topic, ok := juusTopicConfigs[id]
		if !ok {
			reply.ResultList = append(reply.ResultList, *result(errRefused))
			continue
		}
		if _, err := game.DB.ActivateJuusTopic(ctx, db.ActivateJuusTopicParams{CommanderID: client.CommanderID, TopicID: id, ShipGroup: topic.ShipGroup, OpTime: now}); err != nil {
			return err
		}
		reply.ResultList = append(reply.ResultList, *result(nil))
	}
	return client.Send(reply)
}

// handleJuusChatReply records the commander's answer in a chat topic
// (InstagramChatCommand REPLY: chat_id is the line answered, value the
// option); opening a red packet line gives its activity_ins_redpackage
// content.
func (game *Game) handleJuusChatReply(client *connection.Client, req *protobuf.CS_11712) error {
	ctx := context.Background()
	now := uint32(time.Now().Unix())
	reply := &protobuf.SC_11713{Result: result(nil), OpTime: proto.Uint32(now)}
	err := game.tx(ctx, client, func(tx *Tx) error {
		touched, err := tx.TouchJuusTopic(ctx, db.TouchJuusTopicParams{OpTime: now, CommanderID: client.CommanderID, TopicID: req.GetChatGroupId()})
		if err != nil {
			return err
		}
		if touched == 0 {
			reply.Result = result(errRefused)
			return nil
		}
		if err := tx.AddJuusReply(ctx, db.AddJuusReplyParams{CommanderID: client.CommanderID, TopicID: req.GetChatGroupId(), ChatID: req.GetChatId(), Value: req.GetValue()}); err != nil {
			return err
		}
		drop := juusRedPacket(req.GetChatId())
		if drop == nil {
			return nil
		}
		reply.DropList = configDrops([][]uint32{drop})
		return tx.grantDrops(reply.DropList)
	})
	if err != nil {
		return err
	}
	return client.Send(reply)
}

// juusRedPacket is the drop of a red packet chat line, nil for other lines.
func juusRedPacket(chatID uint32) []uint32 {
	word, ok := juusWordConfigs[chatID]
	if !ok || word.Type != juusWordRedPacket {
		return nil
	}
	var id uint64
	switch param := word.Param.(type) {
	case string:
		id, _ = strconv.ParseUint(param, 10, 32)
	case float64:
		id = uint64(param)
	}
	if packet, ok := juusRedPacketConfigs[uint32(id)]; ok && len(packet.Content) >= 3 {
		return packet.Content
	}
	return nil
}

// handleJuusTopicsRead marks chat topics read (InstagramChatCommand
// SET_READTIP), all of them when the list is empty.
func (game *Game) handleJuusTopicsRead(client *connection.Client, req *protobuf.CS_11720) error {
	ctx := context.Background()
	if len(req.GetChatGroupIdList()) == 0 {
		if err := game.DB.ReadAllJuusTopics(ctx, client.CommanderID); err != nil {
			return err
		}
	}
	for _, id := range req.GetChatGroupIdList() {
		if err := game.DB.ReadJuusTopic(ctx, db.ReadJuusTopicParams{CommanderID: client.CommanderID, TopicID: id}); err != nil {
			return err
		}
	}
	return client.Send(&protobuf.SC_11721{Result: result(nil)})
}
