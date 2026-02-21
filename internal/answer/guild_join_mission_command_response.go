package answer

import (
	"encoding/json"
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildJoinMissionCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61007
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61008, err
	}
	response := &protobuf.SC_61008{Result: proto.Uint32(guildEventResultFailure)}
	if client.Commander == nil || payload.GetEventTid() == 0 {
		return client.SendMessage(61008, response)
	}
	shipIDs := payload.GetShipIds()
	if len(shipIDs) == 0 || len(shipIDs) > 4 {
		return client.SendMessage(61008, response)
	}

	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(61008, response)
	}
	event, err := orm.GetGuildOperationEvent(guild.ID, payload.GetEventTid())
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return client.SendMessage(61008, response)
		}
		return 0, 61008, err
	}
	if event.Completed {
		return client.SendMessage(61008, response)
	}

	seen := make(map[uint32]struct{}, len(shipIDs))
	for _, shipID := range shipIDs {
		if shipID == 0 {
			return client.SendMessage(61008, response)
		}
		if _, ok := seen[shipID]; ok {
			return client.SendMessage(61008, response)
		}
		seen[shipID] = struct{}{}
		if _, ok := client.Commander.OwnedShipsMap[shipID]; !ok {
			if _, err := orm.GetOwnedShipByOwnerAndID(client.Commander.CommanderID, shipID); err != nil {
				return client.SendMessage(61008, response)
			}
		}
	}

	person := []guildPersonShipPage{{PageID: 1, ShipIDs: shipIDs}}
	personJSON, err := json.Marshal(person)
	if err != nil {
		return 0, 61008, err
	}
	if err := orm.UpdateGuildOperationEventFormation(guild.ID, payload.GetEventTid(), personJSON, nowUnix()); err != nil {
		return 0, 61008, err
	}
	response.Result = proto.Uint32(guildEventResultSuccess)
	return client.SendMessage(61008, response)
}
