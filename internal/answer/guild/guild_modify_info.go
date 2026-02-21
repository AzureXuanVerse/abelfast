package guild

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func ModifyGuildInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_60026
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 60027, err
	}
	response := protobuf.SC_60027{Result: proto.Uint32(guildResultFailure)}
	operationType := payload.GetType()
	if operationType < 1 || operationType > 5 {
		return client.SendMessage(60027, &response)
	}
	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(60027, &response)
	}
	intValue := payload.GetInt()
	strValue := normalizeGuildText(payload.GetStr())
	if operationType == 1 {
		if !isValidGuildName(strValue) {
			response.Result = proto.Uint32(guildResultNameInvalid)
			return client.SendMessage(60027, &response)
		}
	}
	if operationType == 2 && !isValidGuildFaction(intValue) {
		return client.SendMessage(60027, &response)
	}
	if operationType == 3 && !isValidGuildPolicy(intValue) {
		return client.SendMessage(60027, &response)
	}
	if operationType == 4 && strValue == "" {
		return client.SendMessage(60027, &response)
	}
	nameChangeCost := uint32(0)
	if operationType == 1 {
		cost, err := loadGameSetUint("modify_guild_cost")
		if err != nil {
			return client.SendMessage(60027, &response)
		}
		nameChangeCost = cost
		if !client.Commander.HasEnoughResource(4, nameChangeCost) {
			return client.SendMessage(60027, &response)
		}
	}
	err = orm.UpdateGuildBase(client.Commander, guild.ID, operationType, intValue, strValue, nameChangeCost)
	if errors.Is(err, orm.ErrGuildNameExists) {
		response.Result = proto.Uint32(guildResultNameInvalid)
		return client.SendMessage(60027, &response)
	}
	if err != nil {
		return client.SendMessage(60027, &response)
	}
	if operationType == 1 {
		if resource, ok := client.Commander.OwnedResourcesMap[4]; ok {
			if resource.Amount >= nameChangeCost {
				resource.Amount -= nameChangeCost
			} else {
				resource.Amount = 0
			}
		}
	}
	response.Result = proto.Uint32(guildResultSuccess)
	if _, _, err := client.SendMessage(60027, &response); err != nil {
		return 0, 60027, err
	}
	broadcastGuildBaseUpdate(client, guild.ID)
	return 0, 60027, nil
}
