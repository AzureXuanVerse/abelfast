package answer_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/answer"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestGuildDissolveRemovesGuildChatMessages(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	leaderID := uint32(86401)
	cleanupGuildCoreData(t, leaderID)
	defer cleanupGuildCoreData(t, leaderID)

	leaderClient := &connection.Client{Commander: createGuildCommander(t, leaderID)}
	createBuf, _ := proto.Marshal(&protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(1),
		Name:      proto.String(fmt.Sprintf("CHAT-%d", leaderID)),
		Manifesto: proto.String("chat cleanup"),
	})
	if _, _, err := answer.CreateGuild(&createBuf, leaderClient); err != nil {
		t.Fatalf("CreateGuild failed: %v", err)
	}
	createResp := &protobuf.SC_60002{}
	decodeTestPacket(t, leaderClient, 60002, createResp)
	if createResp.GetResult() != 0 {
		t.Fatalf("expected create success, got %d", createResp.GetResult())
	}
	guildID := createResp.GetId()

	execAnswerExternalTestSQLT(t, "INSERT INTO guild_chat_messages (guild_id, sender_id, sent_at, content) VALUES ($1, $2, NOW(), 'cleanup me')", int64(guildID), int64(leaderID))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_chat_messages (guild_id, sender_id, sent_at, content) VALUES (0, $1, NOW(), 'cleanup placeholder')", int64(leaderID))

	dissolveBuf, _ := proto.Marshal(&protobuf.CS_60010{Id: proto.Uint32(guildID)})
	if _, _, err := answer.GuildDissolve(&dissolveBuf, leaderClient); err != nil {
		t.Fatalf("GuildDissolve failed: %v", err)
	}
	dissolveResp := &protobuf.SC_60011{}
	decodeTestPacket(t, leaderClient, 60011, dissolveResp)
	if dissolveResp.GetResult() != 0 {
		t.Fatalf("expected dissolve success, got %d", dissolveResp.GetResult())
	}

	var count int64
	if err := db.DefaultStore.Pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM guild_chat_messages WHERE guild_id = $1", int64(guildID)).Scan(&count); err != nil {
		t.Fatalf("count guild chat messages: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected dissolved guild chat messages removed, got %d", count)
	}

	if err := db.DefaultStore.Pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM guild_chat_messages WHERE guild_id = 0 AND sender_id = $1", int64(leaderID)).Scan(&count); err != nil {
		t.Fatalf("count placeholder guild chat messages: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected dissolved guild placeholder chat messages removed, got %d", count)
	}
}

func TestGuildChunk2FailurePaths(t *testing.T) {
	orm.InitDatabase()
	seedGuildCoreConfig(t)
	leaderID := uint32(86411)
	deputyID := uint32(86412)
	memberID := uint32(86413)
	cleanupGuildCoreData(t, leaderID, deputyID, memberID)
	defer cleanupGuildCoreData(t, leaderID, deputyID, memberID)

	leaderClient := &connection.Client{Commander: createGuildCommander(t, leaderID)}
	deputyClient := &connection.Client{Commander: createGuildCommander(t, deputyID)}
	createGuildCommander(t, memberID)

	createBuf, _ := proto.Marshal(&protobuf.CS_60001{
		Faction:   proto.Uint32(1),
		Policy:    proto.Uint32(1),
		Name:      proto.String(fmt.Sprintf("FAIL-%d", leaderID)),
		Manifesto: proto.String("failure checks"),
	})
	if _, _, err := answer.CreateGuild(&createBuf, leaderClient); err != nil {
		t.Fatalf("CreateGuild failed: %v", err)
	}
	createResp := &protobuf.SC_60002{}
	decodeTestPacket(t, leaderClient, 60002, createResp)
	if createResp.GetResult() != 0 {
		t.Fatalf("expected create success, got %d", createResp.GetResult())
	}
	guildID := createResp.GetId()
	nowUnix := uint32(time.Now().Unix())
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 2, 0, $3, $3)", int64(guildID), int64(deputyID), int64(nowUnix))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 5, 0, $3, $3)", int64(guildID), int64(memberID), int64(nowUnix))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_user_infos (commander_id, guild_id) VALUES ($1, $2) ON CONFLICT (commander_id) DO UPDATE SET guild_id = EXCLUDED.guild_id", int64(deputyID), int64(guildID))
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_user_infos (commander_id, guild_id) VALUES ($1, $2) ON CONFLICT (commander_id) DO UPDATE SET guild_id = EXCLUDED.guild_id", int64(memberID), int64(guildID))

	setDutyBuf, _ := proto.Marshal(&protobuf.CS_60012{PlayerId: proto.Uint32(deputyID), DutyId: proto.Uint32(99)})
	if _, _, err := answer.SetGuildDuty(&setDutyBuf, leaderClient); err != nil {
		t.Fatalf("SetGuildDuty failed: %v", err)
	}
	setDutyResp := &protobuf.SC_60013{}
	decodeTestPacket(t, leaderClient, 60013, setDutyResp)
	if setDutyResp.GetResult() == 0 {
		t.Fatalf("expected set duty failure for invalid duty")
	}

	fireBuf, _ := proto.Marshal(&protobuf.CS_60014{PlayerId: proto.Uint32(leaderID)})
	if _, _, err := answer.GuildFire(&fireBuf, leaderClient); err != nil {
		t.Fatalf("GuildFire failed: %v", err)
	}
	fireResp := &protobuf.SC_60015{}
	decodeTestPacket(t, leaderClient, 60015, fireResp)
	if fireResp.GetResult() == 0 {
		t.Fatalf("expected fire failure for self-target")
	}

	execAnswerExternalTestSQLT(t, "UPDATE commanders SET last_login = NOW() - INTERVAL '11 days' WHERE commander_id = $1", int64(leaderID))
	execAnswerExternalTestSQLT(t, "UPDATE guilds SET kick_leader_cd = $2 WHERE id = $1", int64(guildID), int64(uint32(time.Now().Unix())+3600))
	impeachBuf, _ := proto.Marshal(&protobuf.CS_60016{PlayerId: proto.Uint32(leaderID)})
	if _, _, err := answer.GuildImpeach(&impeachBuf, deputyClient); err != nil {
		t.Fatalf("GuildImpeach failed: %v", err)
	}
	impeachResp := &protobuf.SC_60017{}
	decodeTestPacket(t, deputyClient, 60017, impeachResp)
	if impeachResp.GetResult() == 0 {
		t.Fatalf("expected impeach failure while cooldown active")
	}

	quitBuf, _ := proto.Marshal(&protobuf.CS_60018{Id: proto.Uint32(guildID)})
	if _, _, err := answer.GuildQuit(&quitBuf, leaderClient); err != nil {
		t.Fatalf("GuildQuit failed: %v", err)
	}
	quitResp := &protobuf.SC_60019{}
	decodeTestPacket(t, leaderClient, 60019, quitResp)
	if quitResp.GetResult() == 0 {
		t.Fatalf("expected quit failure for commander")
	}

	modifyBuf, _ := proto.Marshal(&protobuf.CS_60026{Type: proto.Uint32(2), Int: proto.Uint32(3)})
	if _, _, err := answer.ModifyGuildInfo(&modifyBuf, leaderClient); err != nil {
		t.Fatalf("ModifyGuildInfo failed: %v", err)
	}
	modifyResp := &protobuf.SC_60027{}
	decodeTestPacket(t, leaderClient, 60027, modifyResp)
	if modifyResp.GetResult() == 0 {
		t.Fatalf("expected modify failure for invalid faction")
	}

	dissolveBuf, _ := proto.Marshal(&protobuf.CS_60010{Id: proto.Uint32(guildID + 1)})
	if _, _, err := answer.GuildDissolve(&dissolveBuf, leaderClient); err != nil {
		t.Fatalf("GuildDissolve failed: %v", err)
	}
	dissolveResp := &protobuf.SC_60011{}
	decodeTestPacket(t, leaderClient, 60011, dissolveResp)
	if dissolveResp.GetResult() == 0 {
		t.Fatalf("expected dissolve failure for guild mismatch")
	}
}
