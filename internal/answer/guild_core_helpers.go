package answer

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	guildResultSuccess     = 0
	guildResultFailure     = 1
	guildResultNameInvalid = 2015

	guildMinNameLength = 1
	guildMaxNameLength = 20
)

type guildGameSetEntry struct {
	KeyValue any `json:"key_value"`
}

func parseConfigUint(value any) (uint32, bool) {
	switch v := value.(type) {
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	case uint32:
		return v, true
	case string:
		return 0, false
	default:
		return 0, false
	}
}

func loadGameSetUint(key string) (uint32, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/gameset.json", key)
	if err != nil {
		return 0, err
	}
	var payload guildGameSetEntry
	if err := json.Unmarshal(entry.Data, &payload); err != nil {
		return 0, err
	}
	value, ok := parseConfigUint(payload.KeyValue)
	if !ok {
		return 0, fmt.Errorf("invalid key_value for %s", key)
	}
	return value, nil
}

func normalizeGuildText(value string) string {
	return strings.TrimSpace(value)
}

func isValidGuildName(name string) bool {
	runes := utf8.RuneCountInString(name)
	if runes < guildMinNameLength || runes > guildMaxNameLength {
		return false
	}
	return true
}

func isValidGuildFaction(faction uint32) bool {
	return faction == 1 || faction == 2
}

func isValidGuildPolicy(policy uint32) bool {
	return policy == 1 || policy == 2
}

func buildGuildBaseInfo(guild *orm.Guild) *protobuf.GUILD_BASE_INFO {
	if guild == nil {
		return &protobuf.GUILD_BASE_INFO{
			Id:              proto.Uint32(0),
			Policy:          proto.Uint32(0),
			Faction:         proto.Uint32(0),
			Name:            proto.String(""),
			Level:           proto.Uint32(0),
			Announce:        proto.String(""),
			Manifesto:       proto.String(""),
			Exp:             proto.Uint32(0),
			MemberCount:     proto.Uint32(0),
			ChangeFactionCd: proto.Uint32(0),
			KickLeaderCd:    proto.Uint32(0),
		}
	}
	return &protobuf.GUILD_BASE_INFO{
		Id:              proto.Uint32(guild.ID),
		Policy:          proto.Uint32(guild.Policy),
		Faction:         proto.Uint32(guild.Faction),
		Name:            proto.String(guild.Name),
		Level:           proto.Uint32(guild.Level),
		Announce:        proto.String(guild.Announce),
		Manifesto:       proto.String(guild.Manifesto),
		Exp:             proto.Uint32(guild.Exp),
		MemberCount:     proto.Uint32(guild.MemberCount),
		ChangeFactionCd: proto.Uint32(guild.ChangeFactionCD),
		KickLeaderCd:    proto.Uint32(guild.KickLeaderCD),
	}
}

func buildGuildExpansionInfo(guild *orm.Guild) *protobuf.GUILD_EXPANSION_INFO {
	capital := uint32(0)
	if guild != nil {
		capital = guild.Capital
	}
	return &protobuf.GUILD_EXPANSION_INFO{
		Capital: proto.Uint32(capital),
		ThisWeeklyTasks: &protobuf.WEEKLY_TASK{
			Id:            proto.Uint32(0),
			Progress:      proto.Uint32(0),
			Monday_0Clock: proto.Uint32(0),
		},
		BenefitFinishTime:     proto.Uint32(0),
		RetreatCnt:            proto.Uint32(0),
		TechCancelCnt:         proto.Uint32(0),
		LastBenefitFinishTime: proto.Uint32(0),
		ActiveEventCnt:        proto.Uint32(0),
	}
}

func buildGuildMemberInfo(member orm.GuildMember) *protobuf.MEMBER_INFO {
	preOnline := member.PreOnlineTime
	if preOnline == 0 {
		preOnline = uint32(member.LastLogin.Unix())
	}
	return &protobuf.MEMBER_INFO{
		Liveness:      proto.Uint32(member.Liveness),
		Duty:          proto.Uint32(member.Duty),
		Id:            proto.Uint32(member.CommanderID),
		Name:          proto.String(member.CommanderName),
		Lv:            proto.Uint32(member.CommanderLevel),
		Adv:           proto.String(member.Manifesto),
		Online:        proto.Uint32(0),
		PreOnlineTime: proto.Uint32(preOnline),
		Display: &protobuf.DISPLAYINFO{
			Icon:          proto.Uint32(member.DisplayIconID),
			Skin:          proto.Uint32(member.DisplaySkinID),
			IconFrame:     proto.Uint32(member.IconFrameID),
			ChatFrame:     proto.Uint32(member.ChatFrameID),
			IconTheme:     proto.Uint32(member.IconThemeID),
			MarryFlag:     proto.Uint32(0),
			TransformFlag: proto.Uint32(0),
		},
		JoinTime: proto.Uint32(member.JoinTime),
	}
}

func broadcastGuildBaseUpdate(client *connection.Client, guildID uint32) {
	if client == nil || client.Server == nil {
		return
	}
	guild, err := orm.GetGuildByID(guildID)
	if err != nil {
		return
	}
	packet := &protobuf.SC_60030{Guild: buildGuildBaseInfo(guild)}
	for _, connected := range client.Server.ListClients() {
		if connected == nil || connected.Commander == nil {
			continue
		}
		membership, err := orm.GetCommanderGuildMembership(connected.Commander.CommanderID)
		if err != nil {
			continue
		}
		if membership.GuildID != guildID {
			continue
		}
		_, _, _ = connected.SendMessage(60030, packet)
	}
}
