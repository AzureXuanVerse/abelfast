package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func HandleIslandCollectionComplete(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_21533
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 21534, err
	}

	response := &protobuf.SC_21534{Result: proto.Uint32(1)}
	if request.GetCollectId() == 0 {
		return client.SendMessage(21534, response)
	}

	template, err := loadIslandCollectionTemplate(request.GetCollectId())
	if err != nil || len(template.FragmentList) == 0 {
		return client.SendMessage(21534, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		alreadyCompleted, err := orm.IsIslandCollectionCompletedTx(context.Background(), tx, client.Commander.CommanderID, request.GetCollectId())
		if err != nil {
			return err
		}
		if alreadyCompleted {
			return nil
		}

		for _, fragmentID := range template.FragmentList {
			hasFragment, err := orm.HasIslandCollectFragmentTx(context.Background(), tx, client.Commander.CommanderID, fragmentID)
			if err != nil {
				return err
			}
			if !hasFragment {
				return nil
			}
		}

		marked, err := orm.MarkIslandCollectionCompletedTx(context.Background(), tx, client.Commander.CommanderID, request.GetCollectId())
		if err != nil {
			return err
		}
		if marked {
			response.Result = proto.Uint32(0)
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(21534, response)
	}

	return client.SendMessage(21534, response)
}
