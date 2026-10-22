package game

import (
	"fmt"
	"strings"

	"github.com/ggmolly/belfast/internal/db"
)

// CheckNotice refuses what the client would skip (NewBulletinBoardLayer:
// checkNotice) or show without a label.
func CheckNotice(notice db.Notice) error {
	if notice.Title == "" {
		return fmt.Errorf("notice without a title")
	}
	switch notice.Kind {
	case NoticeGateway:
		return nil
	case NoticeBoard:
	default:
		return fmt.Errorf("kind %q: want %q or %q", notice.Kind, NoticeBoard, NoticeGateway)
	}
	if notice.TagType < 1 || notice.TagType > 3 {
		return fmt.Errorf("tag %d: want 1 (event), 2 (system) or 3 (news)", notice.TagType)
	}
	if notice.Icon < 1 || notice.Icon > 9 {
		return fmt.Errorf("icon %d: want 1 to 9", notice.Icon)
	}
	if strings.HasPrefix(notice.TitleImage, "<config") && !strings.Contains(notice.TitleImage, "/>") {
		return fmt.Errorf("image %q: <config ... /> isn't closed", notice.TitleImage)
	}
	return nil
}

// NoticeParams is what CreateNotice stores of a notice.
func NoticeParams(notice db.Notice) db.CreateNoticeParams {
	return db.CreateNoticeParams{
		Kind:       notice.Kind,
		Version:    notice.Version,
		BtnTitle:   notice.BtnTitle,
		Title:      notice.Title,
		TitleImage: notice.TitleImage,
		TimeDesc:   notice.TimeDesc,
		Content:    notice.Content,
		TagType:    notice.TagType,
		Icon:       notice.Icon,
		Track:      notice.Track,
		Priority:   notice.Priority,
		NeedLevel:  notice.NeedLevel,
	}
}
