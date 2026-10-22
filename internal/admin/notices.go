package admin

import (
	"net/http"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
)

type noticeRow struct {
	ID         uint32 `json:"id"`
	Kind       string `json:"kind"`
	Version    uint32 `json:"version"`
	Button     string `json:"button"`
	Title      string `json:"title"`
	TitleImage string `json:"image"`
	TimeDesc   string `json:"time"`
	Content    string `json:"content"`
	Tag        uint32 `json:"tag"`
	Icon       uint32 `json:"icon"`
	Priority   uint32 `json:"priority"`
	NeedLevel  uint32 `json:"need_level"`
}

// handleNotices lists the bulletin board and gateway notices.
func (server *Server) handleNotices(r *http.Request) (any, error) {
	notices, err := server.Store.ListNotices(r.Context())
	if err != nil {
		return nil, err
	}
	rows := make([]noticeRow, 0, len(notices))
	for _, n := range notices {
		rows = append(rows, noticeRow{
			ID: n.ID, Kind: n.Kind, Version: n.Version, Button: n.BtnTitle, Title: n.Title, TitleImage: n.TitleImage,
			TimeDesc: n.TimeDesc, Content: n.Content, Tag: n.TagType, Icon: n.Icon, Priority: n.Priority, NeedLevel: n.NeedLevel,
		})
	}
	return rows, nil
}

// handleAddNotice stores a notice with `belfast notice add`'s defaults;
// clients see it at their next login.
func (server *Server) handleAddNotice(r *http.Request) (any, error) {
	body := noticeRow{Kind: game.NoticeBoard, Tag: 1, Icon: 7}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	notice := db.Notice{
		Kind: body.Kind, Version: 1, BtnTitle: body.Button, Title: body.Title, TitleImage: body.TitleImage,
		TimeDesc: body.TimeDesc, Content: body.Content, TagType: body.Tag, Icon: body.Icon, Priority: body.Priority, NeedLevel: body.NeedLevel,
	}
	if notice.BtnTitle == "" {
		notice.BtnTitle = notice.Title
	}
	if notice.TimeDesc == "" {
		notice.TimeDesc = time.Now().Format("1/2/2006")
	}
	if err := game.CheckNotice(notice); err != nil {
		return nil, badRequest(err.Error())
	}
	id, err := server.Store.CreateNotice(r.Context(), game.NoticeParams(notice))
	if err != nil {
		return nil, err
	}
	return map[string]uint32{"id": id}, nil
}

func (server *Server) handleDeleteNotice(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	deleted, err := server.Store.DeleteNotice(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return nil, notFound("no notice " + itoa(id))
	}
	return struct{}{}, nil
}
