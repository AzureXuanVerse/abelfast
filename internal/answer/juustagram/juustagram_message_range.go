package juustagram

import (
	"errors"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func JuustagramMessageRange(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_11705
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, consts.JuustagramPacketRangeResp, err
	}
	if client.Commander == nil {
		return 0, consts.JuustagramPacketRangeResp, errors.New("missing commander")
	}
	// SC_11700 remains intentionally unsupported because current client bootstrap
	// flow requests Juustagram messages via CS_11705 and consumes SC_11706.
	messages, err := buildJuustagramMessagesForIDs(client.Commander.CommanderID, payload.GetIdList())
	if err != nil {
		return 0, consts.JuustagramPacketRangeResp, err
	}
	response := protobuf.SC_11706{InsMessageList: messages}
	return client.SendMessage(consts.JuustagramPacketRangeResp, &response)
}

func buildJuustagramMessagesForIDs(commanderID uint32, ids []uint32) ([]*protobuf.INS_MESSAGE, error) {
	templates := make([]orm.JuustagramTemplate, 0, len(ids))
	for _, id := range ids {
		template, err := orm.GetJuustagramTemplate(id)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				continue
			}
			return nil, err
		}
		templates = append(templates, *template)
	}
	now := uint32(time.Now().Unix())
	messages := make([]*protobuf.INS_MESSAGE, 0, len(templates))
	for _, template := range templates {
		if ok, err := isPublishableJuustagramTemplate(template); err != nil {
			return nil, err
		} else if !ok {
			// Skip templates that are not ready to be served to clients.
			logJuustagramSkip(template, commanderID)
			continue
		}
		message, err := BuildJuustagramMessage(commanderID, template.ID, now)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, nil
}
