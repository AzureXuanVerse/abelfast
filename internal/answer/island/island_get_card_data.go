package island

import (
	"strconv"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandGetCardData(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21326
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21327, err
	}

	targetID := payload.GetUserId()
	if targetID == 0 {
		targetID = client.Commander.CommanderID
	}

	targetCommander, err := orm.LoadCommanderWithDetails(targetID)
	if err != nil {
		if db.IsNotFound(err) {
			return client.SendMessage(21327, emptyIslandCardDataResponse())
		}
		return 0, 21327, err
	}

	cardState, err := orm.GetIslandCardState(targetID)
	if err != nil {
		if !db.IsNotFound(err) {
			return 0, 21327, err
		}
		cardState = orm.NewIslandCardState(targetID)
	}

	achievementState, err := orm.GetIslandAchievementState(targetID)
	if err != nil && !db.IsNotFound(err) {
		return 0, 21327, err
	}
	if achievementState != nil {
		cardState.AchievementTotal = uint32(len(achievementState.FinishList))
	}

	bookState, err := orm.GetIslandBookState(targetID)
	if err == nil {
		cardState.BookNum = uint32(len(bookState.BookList))
	}

	cardState.ShipNum = uint32(len(targetCommander.OwnedShipsMap))

	isSelf := targetID == client.Commander.CommanderID
	goodFlag := uint32(0)
	labelFlag := uint32(0)
	if !isSelf {
		liked, likeErr := orm.HasIslandCardLike(client.Commander.CommanderID, targetID)
		if likeErr == nil && liked {
			goodFlag = 1
		}
		gifted, giftErr := orm.HasIslandCardLabelGift(client.Commander.CommanderID, targetID)
		if giftErr == nil && gifted {
			labelFlag = 1
		}
		cardState.VisitNum++
		if err := orm.UpsertIslandCardState(cardState); err != nil {
			return 0, 21327, err
		}
	}

	labels := buildIslandLabelList(cardState.LabelCounts)
	if !isSelf && cardState.LabelViewFlag == 0 {
		labels = []*protobuf.PB_ISLAND_LABEL{}
	}

	picture := cardState.Picture
	if picture == "" {
		picture = strconv.FormatUint(uint64(loadIslandSetInt("island_card_photo_default", 4001)), 10)
	}

	response := &protobuf.SC_21327{
		Name:          proto.String(targetCommander.Name),
		Picture:       proto.String(picture),
		VisitWord:     proto.String(cardState.VisitWord),
		Lv:            proto.Uint32(uint32(targetCommander.Level)),
		SocialFlag:    proto.Uint32(cardState.SocialFlag),
		LabelViewFlag: proto.Uint32(cardState.LabelViewFlag),
		LabelList:     labels,
		AchieveList:   append([]uint32(nil), cardState.AchieveDisplayIDs...),
		AchieveNum:    proto.Uint32(cardState.AchievementTotal),
		VisitNum:      proto.Uint32(cardState.VisitNum),
		GoodNum:       proto.Uint32(cardState.GoodNum),
		ShipNum:       proto.Uint32(cardState.ShipNum),
		BookNum:       proto.Uint32(cardState.BookNum),
		LabelFlag:     proto.Uint32(labelFlag),
		GoodFlag:      proto.Uint32(goodFlag),
		WhiteFlag:     proto.Uint32(0),
		BlackFlag:     proto.Uint32(0),
	}
	return client.SendMessage(21327, response)
}

func emptyIslandCardDataResponse() *protobuf.SC_21327 {
	picture := strconv.FormatUint(uint64(loadIslandSetInt("island_card_photo_default", 4001)), 10)
	return &protobuf.SC_21327{
		Name:          proto.String(""),
		Picture:       proto.String(picture),
		VisitWord:     proto.String(""),
		Lv:            proto.Uint32(0),
		SocialFlag:    proto.Uint32(0),
		LabelViewFlag: proto.Uint32(0),
		LabelList:     []*protobuf.PB_ISLAND_LABEL{},
		AchieveList:   []uint32{},
		AchieveNum:    proto.Uint32(0),
		VisitNum:      proto.Uint32(0),
		GoodNum:       proto.Uint32(0),
		ShipNum:       proto.Uint32(0),
		BookNum:       proto.Uint32(0),
		LabelFlag:     proto.Uint32(0),
		GoodFlag:      proto.Uint32(0),
		WhiteFlag:     proto.Uint32(0),
		BlackFlag:     proto.Uint32(0),
	}
}
