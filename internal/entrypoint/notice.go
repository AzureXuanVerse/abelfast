package entrypoint

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
)

// noticeCommand is `belfast notice <add|list|update|delete> [flags]`: it
// manages belfast's announcements in the database, next to a running server
// or not. The client gets them at its next login: the bulletin board in the
// login burst (SC_11300), the gateway popups in SC_10021.
func noticeCommand(dbPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notice",
		Short: "Manage the bulletin board and gateway notices",
		Long: `Manage belfast's announcements. Clients see the changes at their next
login (bulletin board: game server login, gateway notices: account login).`,
	}
	subcommand := func(name, short string, fields, id bool) *cobra.Command {
		var opts noticeOptions
		sub := &cobra.Command{
			Use:   name,
			Short: short,
			Args:  cobra.NoArgs,
			Run: func(sub *cobra.Command, args []string) {
				runNotice(name, *dbPath, sub, opts)
			},
		}
		flags := sub.Flags()
		if id {
			flags.UintVar(&opts.id, "id", 0, "notice id")
			sub.MarkFlagRequired("id")
		}
		if fields {
			flags.StringVar(&opts.kind, "kind", game.NoticeBoard, `"board" (in-game bulletin board) or "gateway" (popup after the account login, title and content only)`)
			flags.StringVar(&opts.title, "title", "", `title, "list title&page title" to have the board show two`)
			flags.StringVar(&opts.button, "button", "", "the board's tab label, the title when empty")
			flags.StringVar(&opts.content, "content", "", `rich text (<size=35>, <color=#rrggbb>, <b>...), <banner>image url</banner> for an image, <segment/> to split; "\n" for a new line`)
			flags.StringVar(&opts.contentFile, "content-file", "", "read the content from this file instead")
			flags.StringVar(&opts.image, "image", "", "title image url, optionally prefixed by a jump: <config type = 1 param = 'url' /> opens a url, type = 2 param = 'scene name' a scene, type = 3 param = N an activity; the board then shows only the image")
			flags.StringVar(&opts.timeDesc, "time", "", "date shown under the title, today (M/D/YYYY) when empty on add")
			flags.UintVar(&opts.tag, "tag", 1, "board tab: 1 event, 2 system, 3 news")
			flags.UintVar(&opts.icon, "icon", 7, "tab icon: 1 activity_common, 2 activity_summary, 3 activity_time_limit, 4 build_time_limit, 5 equibment_skin_new, 6 furniture_new, 7 info_common, 8 skin_new, 9 system_common")
			flags.UintVar(&opts.priority, "priority", 0, "order, highest first; the highest one's tab opens first")
			flags.UintVar(&opts.needLevel, "need-level", 0, "shown only to commanders above this level")
			flags.StringVar(&opts.track, "track", "", "tracking tag sent to the SDK when a type 1 image link is opened")
		}
		if name == "update" {
			flags.BoolVar(&opts.unread, "unread", false, "give the notice a new id so its red dot shows again")
		}
		return sub
	}
	cmd.AddCommand(
		subcommand("add", "Store a new notice", true, false),
		subcommand("list", "List the stored notices", false, false),
		subcommand("update", "Change the given fields, bumping the version", true, true),
		subcommand("delete", "Remove a notice", false, true),
	)
	return cmd
}

type noticeOptions struct {
	id                                                                uint
	kind, title, button, content, contentFile, image, timeDesc, track string
	tag, icon, priority, needLevel                                    uint
	unread                                                            bool
}

func runNotice(command, dbPath string, cmd *cobra.Command, opts noticeOptions) {
	set := map[string]bool{}
	cmd.Flags().Visit(func(f *pflag.Flag) { set[f.Name] = true })
	if opts.contentFile != "" {
		data, err := os.ReadFile(opts.contentFile)
		if err != nil {
			fatal(err)
		}
		opts.content = string(data)
		set["content"] = true
	} else {
		opts.content = strings.ReplaceAll(opts.content, `\n`, "\n")
	}

	store, err := db.Open(dbPath)
	if err != nil {
		fatal(fmt.Errorf("database %s: %w", dbPath, err))
	}
	ctx := context.Background()
	// apply copies the flags given on the command line onto notice
	apply := func(notice *db.Notice) {
		strs := map[string]struct{ field, value *string }{
			"kind": {&notice.Kind, &opts.kind}, "title": {&notice.Title, &opts.title}, "button": {&notice.BtnTitle, &opts.button},
			"content": {&notice.Content, &opts.content}, "image": {&notice.TitleImage, &opts.image}, "time": {&notice.TimeDesc, &opts.timeDesc},
			"track": {&notice.Track, &opts.track},
		}
		for name, f := range strs {
			if set[name] {
				*f.field = *f.value
			}
		}
		uints := map[string]struct {
			field *uint32
			value *uint
		}{
			"tag": {&notice.TagType, &opts.tag}, "icon": {&notice.Icon, &opts.icon}, "priority": {&notice.Priority, &opts.priority},
			"need-level": {&notice.NeedLevel, &opts.needLevel},
		}
		for name, f := range uints {
			if set[name] {
				*f.field = uint32(*f.value)
			}
		}
	}

	switch command {
	case "add":
		notice := db.Notice{Kind: opts.kind, Version: 1, TagType: uint32(opts.tag), Icon: uint32(opts.icon), TimeDesc: time.Now().Format("1/2/2006")}
		apply(&notice)
		if notice.BtnTitle == "" {
			notice.BtnTitle = notice.Title
		}
		if err := checkNotice(notice); err != nil {
			fatal(err)
		}
		id, err := store.CreateNotice(ctx, createNoticeParams(notice))
		if err != nil {
			fatal(err)
		}
		fmt.Printf("%s notice %d added\n", notice.Kind, id)
	case "list":
		notices, err := store.ListNotices(ctx)
		if err != nil {
			fatal(err)
		}
		for _, notice := range notices {
			fmt.Printf("%d\t%s\tv%d\ttag %d\ticon %d\tpriority %d\tlevel>%d\t%s\t%q\t%q\t%d bytes\n", notice.ID, notice.Kind, notice.Version, notice.TagType, notice.Icon, notice.Priority, notice.NeedLevel, notice.TimeDesc, notice.BtnTitle, notice.Title, len(notice.Content))
			if notice.TitleImage != "" {
				fmt.Printf("\timage %s\n", notice.TitleImage)
			}
		}
	case "update":
		notice, err := store.GetNotice(ctx, uint32(opts.id))
		if err != nil {
			fatal(fmt.Errorf("notice %d: %w", opts.id, err))
		}
		apply(&notice)
		notice.Version++
		if err := checkNotice(notice); err != nil {
			fatal(err)
		}
		newID := notice.ID
		err = store.InTx(ctx, func(queries *db.Queries) error {
			if !opts.unread {
				params := createNoticeParams(notice)
				return queries.UpdateNotice(ctx, db.UpdateNoticeParams{
					Kind: params.Kind, Version: params.Version, BtnTitle: params.BtnTitle, Title: params.Title,
					TitleImage: params.TitleImage, TimeDesc: params.TimeDesc, Content: params.Content,
					TagType: params.TagType, Icon: params.Icon, Track: params.Track, Priority: params.Priority,
					NeedLevel: params.NeedLevel, ID: notice.ID,
				})
			}
			if _, err := queries.DeleteNotice(ctx, notice.ID); err != nil {
				return err
			}
			newID, err = queries.CreateNotice(ctx, createNoticeParams(notice))
			return err
		})
		if err != nil {
			fatal(err)
		}
		if newID != notice.ID {
			fmt.Printf("notice %d updated to version %d, now notice %d\n", notice.ID, notice.Version, newID)
		} else {
			fmt.Printf("notice %d updated to version %d\n", notice.ID, notice.Version)
		}
	case "delete":
		deleted, err := store.DeleteNotice(ctx, uint32(opts.id))
		if err != nil {
			fatal(err)
		}
		if deleted == 0 {
			fatal(fmt.Errorf("no notice %d", opts.id))
		}
		fmt.Printf("notice %d deleted\n", opts.id)
	}
}

// checkNotice refuses what the client would skip (NewBulletinBoardLayer:
// checkNotice) or show without a label.
func checkNotice(notice db.Notice) error {
	if notice.Title == "" {
		return fmt.Errorf("notice without a title")
	}
	switch notice.Kind {
	case game.NoticeGateway:
		return nil
	case game.NoticeBoard:
	default:
		return fmt.Errorf("kind %q: want %q or %q", notice.Kind, game.NoticeBoard, game.NoticeGateway)
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

func createNoticeParams(notice db.Notice) db.CreateNoticeParams {
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
