package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
)

type guildEventContext struct {
	GuildID     uint32
	CommanderID uint32
	Duty        uint32
	OperationID uint32
}

func activeGuildEventContext(commanderID uint32) (*guildEventContext, error) {
	guild, member, err := orm.GetGuildForCommander(commanderID)
	if err != nil {
		return nil, err
	}
	state, err := orm.GetGuildOperationState(guild.ID)
	if err != nil {
		return nil, err
	}
	if state.EndTime <= nowUnix() {
		return nil, db.ErrNotFound
	}
	return &guildEventContext{
		GuildID:     guild.ID,
		CommanderID: commanderID,
		Duty:        member.Duty,
		OperationID: state.ChapterID,
	}, nil
}

func isGuildAdminDuty(duty uint32) bool {
	return duty == orm.GuildDutyCommander || duty == orm.GuildDutyDeputy
}

func isGuildEventContextError(err error) bool {
	return errors.Is(err, db.ErrNotFound) || errors.Is(err, orm.ErrCommanderNotInGuild)
}
