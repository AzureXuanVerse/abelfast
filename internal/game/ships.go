package game

import (
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// A ship as it is obtained: level 1, full morale (ship_data_template.energy is
// 150 for every ship), 50.00 affinity and capped at 70 until limit broken
// (ship_data_template.max_level at 0 breaks).
const (
	newShipEnergy   = 150
	newShipIntimacy = 5000
	newShipMaxLevel = 70
)

// NewShip returns the row for a freshly obtained ship.
func NewShip(commanderID uint32, templateID uint32) db.CreateShipParams {
	return db.CreateShipParams{
		CommanderID: commanderID,
		TemplateID:  templateID,
		Level:       1,
		Exp:         0,
		Energy:      newShipEnergy,
		Intimacy:    newShipIntimacy,
		MaxLevel:    newShipMaxLevel,
		IsLocked:    false,
		CreatedAt:   time.Now().Unix(),
	}
}

// shipInfo is a SHIPINFO, read by BayProxy into a Ship (model/vo/ship.lua).
func shipInfo(ship db.Ship) *protobuf.SHIPINFO {
	locked := uint32(0)
	if ship.IsLocked {
		locked = 1
	}
	return &protobuf.SHIPINFO{
		Id:          proto.Uint32(ship.ID),
		TemplateId:  proto.Uint32(ship.TemplateID),
		Level:       proto.Uint32(ship.Level),
		Exp:         proto.Uint32(ship.Exp),
		Energy:      proto.Uint32(ship.Energy),
		State:       proto.Uint32(0),
		IsLocked:    proto.Uint32(locked),
		Intimacy:    proto.Uint32(ship.Intimacy),
		Proficiency: proto.Uint32(0),
		CreateTime:  proto.Uint32(uint32(ship.CreatedAt)),
		SkinId:      proto.Uint32(0),
		Propose:     proto.Uint32(0),
		MaxLevel:    proto.Uint32(ship.MaxLevel),
		ActivityNpc: proto.Uint32(0),
	}
}
