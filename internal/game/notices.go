package game

import (
	"context"
	"strconv"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Announcements are belfast's own, stored in the notices table and managed
// with `belfast notice`; the client asks for none, it gets them pushed.
//
// The bulletin board (SC_11300, model/proxy/servernoticeproxy.lua,
// view/notice/newbulletinboardlayer.lua) merges a push into what it has by
// id, so the login burst lists them all. A notice shows when the commander's
// level is above need_level (ServerNotice:ShouldShow) and its tag_type is a
// tab, 1 event, 2 system, 3 news (NewBulletinBoardLayer:checkNotice); the tab
// of the highest priority opens first. Its tab's red dot stays off once the
// id was opened (PlayerPrefs "ServerNotice<id>"), while the main screen's
// button tip and the board's auto-open come back whenever a notice's id,
// title or content changes (ServerNoticeProxy:getUniqueCode). version only
// keys the client's cache of the title image and banners (BulletinBoardMgr).
//
// The gateway popup (SC_10021 notice_list, GatewayNoticeLayer) shows every
// gateway notice, title and rich text content, one after the other after
// each account login (LoginMediator on GAME.USER_LOGIN_SUCCESS).

// Notice kinds, the notices.kind column.
const (
	NoticeBoard   = "board"
	NoticeGateway = "gateway"
)

// boardNotices is the login burst's SC_11300.
func boardNotices(ctx context.Context, queries *db.Queries) (*protobuf.SC_11300, error) {
	notices, err := queries.ListNoticesByKind(ctx, NoticeBoard)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_11300{NoticeList: make([]*protobuf.NOTICEINFO_P11, len(notices))}
	for i, notice := range notices {
		msg.NoticeList[i] = boardNoticeInfo(notice)
	}
	return msg, nil
}

// boardNoticeInfo is a notice as the bulletin board reads it.
func boardNoticeInfo(notice db.Notice) *protobuf.NOTICEINFO_P11 {
	return &protobuf.NOTICEINFO_P11{
		Id:         proto.Uint32(notice.ID),
		Version:    proto.String(strconv.FormatUint(uint64(notice.Version), 10)),
		BtnTitle:   proto.String(notice.BtnTitle),
		Title:      proto.String(notice.Title),
		TitleImage: proto.String(notice.TitleImage),
		TimeDesc:   proto.String(notice.TimeDesc),
		Content:    proto.String(notice.Content),
		TagType:    proto.Uint32(notice.TagType),
		Icon:       proto.Uint32(notice.Icon),
		Track:      proto.String(notice.Track),
		Priority:   proto.Uint32(notice.Priority),
		NeedLevel:  proto.Uint32(notice.NeedLevel),
	}
}

// GatewayNotices is SC_10021's notice_list.
func GatewayNotices(ctx context.Context, queries *db.Queries) ([]*protobuf.NOTICEINFO_P10, error) {
	notices, err := queries.ListNoticesByKind(ctx, NoticeGateway)
	if err != nil {
		return nil, err
	}
	list := make([]*protobuf.NOTICEINFO_P10, len(notices))
	for i, notice := range notices {
		list[i] = &protobuf.NOTICEINFO_P10{
			Id:      proto.Uint32(notice.ID),
			Title:   proto.String(notice.Title),
			Content: proto.String(notice.Content),
		}
	}
	return list, nil
}
