package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// const.lua DROP_TYPE_*: the frames AttireProxy keeps (model/vo/attire)
const (
	dropTypeIconFrame = 14 // item_data_frame
	dropTypeChatFrame = 15 // item_data_chat
)

// attireFrames is SC_11003's icon_frame_list and chat_frame_list
// (AttireProxy): time is a timed frame's end (AttireFrame.updateData), 0 for
// one owned for good, as every stored frame is.
func attireFrames(ctx context.Context, queries *db.Queries, commanderID uint32) (icons, chats []*protobuf.IDTIMEINFO, err error) {
	frames, err := queries.ListAttireFrames(ctx, commanderID)
	if err != nil {
		return nil, nil, err
	}
	for _, frame := range frames {
		info := &protobuf.IDTIMEINFO{Id: proto.Uint32(frame.FrameID), Time: proto.Uint32(0)}
		if frame.DropType == dropTypeIconFrame {
			icons = append(icons, info)
		} else {
			chats = append(chats, info)
		}
	}
	return icons, chats, nil
}

// addAttireFrame stores a frame drop (DROP_TYPE_ICON_FRAME or
// DROP_TYPE_CHAT_FRAME) for good; the client adds it itself
// (AttireProxy.addAttireFrame).
func (tx *Tx) addAttireFrame(kind, id uint32) error {
	return tx.AddAttireFrame(tx.ctx, db.AddAttireFrameParams{CommanderID: tx.commanderID, DropType: kind, FrameID: id})
}
