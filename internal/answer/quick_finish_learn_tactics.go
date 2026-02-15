package answer

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	lessonQuickFinishResultOK                = 0
	lessonQuickFinishResultInvalidRoom       = 1
	lessonQuickFinishResultSessionNotFound   = 2
	lessonQuickFinishResultAllowanceExceeded = 3
	lessonQuickFinishResultInvalidState      = 4
)

var (
	errQuickFinishSessionNotFound = errors.New("quick finish session not found")
	errQuickFinishInvalidState    = errors.New("quick finish invalid state")
)

func QuickFinishLearnTactics(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_22014
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 22015, err
	}

	response := protobuf.SC_22015{Result: proto.Uint32(lessonQuickFinishResultInvalidRoom)}
	roomID := payload.GetRoomid()
	if roomID == 0 {
		return client.SendMessage(22015, &response)
	}

	if client.Commander.OwnedShipsMap == nil {
		if err := client.Commander.Load(); err != nil {
			return 0, 22015, err
		}
	}

	now := time.Now().UTC()
	ctx := context.Background()
	err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		lesson, err := orm.GetCommanderSkillClassByRoomTx(ctx, tx, client.Commander.CommanderID, roomID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return errQuickFinishSessionNotFound
			}
			return err
		}

		allowance, err := orm.GetCommanderSkillLearnTimeAllowance(client.Commander.CommanderID, now)
		if err != nil {
			return err
		}
		if allowance == 0 {
			return orm.ErrNoQuickFinishAllowance
		}

		skillCfg, err := loadSkillTemplate(lesson.SkillID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return errQuickFinishInvalidState
			}
			return err
		}

		shipSkill, err := orm.GetOrCreateCommanderShipSkillTx(ctx, tx, client.Commander.CommanderID, lesson.ShipID, lesson.SkillPos, lesson.SkillID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return errQuickFinishInvalidState
			}
			return err
		}

		applyLessonExp(shipSkill, lesson.Exp, skillCfg.MaxLevel)
		if err := orm.SaveCommanderShipSkillTx(ctx, tx, shipSkill); err != nil {
			return err
		}

		if err := orm.DeleteCommanderSkillClassTx(ctx, tx, client.Commander.CommanderID, roomID); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return errQuickFinishSessionNotFound
			}
			return err
		}

		_, err = orm.ConsumeCommanderQuickFinishTx(ctx, tx, client.Commander.CommanderID, allowance, now)
		return err
	})
	if err != nil {
		switch {
		case errors.Is(err, orm.ErrNoQuickFinishAllowance):
			response.Result = proto.Uint32(lessonQuickFinishResultAllowanceExceeded)
			return client.SendMessage(22015, &response)
		case errors.Is(err, errQuickFinishSessionNotFound):
			response.Result = proto.Uint32(lessonQuickFinishResultSessionNotFound)
			return client.SendMessage(22015, &response)
		case errors.Is(err, errQuickFinishInvalidState):
			response.Result = proto.Uint32(lessonQuickFinishResultInvalidState)
			return client.SendMessage(22015, &response)
		default:
			return 0, 22015, err
		}
	}

	response.Result = proto.Uint32(lessonQuickFinishResultOK)
	return client.SendMessage(22015, &response)
}
