package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func CreateGuild(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_60001
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 60002, err
	}
	response := protobuf.SC_60002{Result: proto.Uint32(guildResultFailure), Id: proto.Uint32(0)}
	faction := payload.GetFaction()
	policy := payload.GetPolicy()
	name := normalizeGuildText(payload.GetName())
	manifesto := normalizeGuildText(payload.GetManifesto())
	if !isValidGuildFaction(faction) || !isValidGuildPolicy(policy) || !isValidGuildName(name) || manifesto == "" {
		response.Result = proto.Uint32(guildResultNameInvalid)
		return client.SendMessage(60002, &response)
	}
	createCost, err := loadGameSetUint("create_guild_cost")
	if err != nil {
		return client.SendMessage(60002, &response)
	}
	if !client.Commander.HasEnoughResource(4, createCost) {
		return client.SendMessage(60002, &response)
	}
	baseCapital, err := orm.GetGuildSetUint("base_capital")
	if err != nil {
		return client.SendMessage(60002, &response)
	}
	defaultTechID, err := orm.GetGuildSetUint("guild_tech_default")
	if err != nil {
		return client.SendMessage(60002, &response)
	}
	guildID, err := orm.CreateGuild(client.Commander, faction, policy, name, manifesto, createCost, baseCapital, defaultTechID)
	if errors.Is(err, orm.ErrGuildNameExists) {
		response.Result = proto.Uint32(guildResultNameInvalid)
		return client.SendMessage(60002, &response)
	}
	if err != nil {
		return client.SendMessage(60002, &response)
	}
	response.Result = proto.Uint32(guildResultSuccess)
	response.Id = proto.Uint32(guildID)
	return client.SendMessage(60002, &response)
}
