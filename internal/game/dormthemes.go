package game

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The backyard's theme templates (view/backyard/themetemplate/): the
// commander's own (custom, dorm_themes, by slot), and the theme shop, where
// every commander of the server sees the templates others shared, likes them
// and keeps a collection of them. None of it is in an official capture: the
// replies are built from the Lua alone. The EN client hides the template
// scene (LOCK_BACKYARD_TEMPLATE, const.lua), so only raw packets reach the
// shop there. Belfast has no picture storage (the client uploads the
// previews to an OSS bucket, OSSMgr, only on CN), so the previews are the
// md5 the author's client saved them with.

const (
	// BackYardConst.MAX_UPLOAD_THEME_CNT, MAX_COLLECTION_CNT
	maxSharedThemes    = 2
	maxCollectedThemes = 30

	// the theme shop's orders (CS_19117 typ, BackYardConst.ThemeSortIndex2ServerIndex)
	themeShopOldest   = 1
	themeShopNewest   = 2
	themeShopHottest  = 3
	themeShopColdest  = 4
	themeShopRandom   = 5
	themeShopMaxPages = 10 // DormProxy.MAX_PAGE
)

// errThemeGone is a shared template that was deleted or stopped being
// shared (result 20, Backyard_theme_template_be_delete_tip /
// backyard_not_found_theme_template).
var errThemeGone error = refusal{"no such shared theme", 20}

// themeID is a template's DORMTHEME id (BackYardBaseThemeTemplate.BuildId).
func themeID(commanderID, pos uint32) string {
	return fmt.Sprintf("%d%d", commanderID, pos)
}

// dormThemes are the commander's theme templates, by slot, with whether
// they are shared and their likes and collections while they are.
func dormThemes(ctx context.Context, queries *db.Queries, commanderID uint32) ([]*protobuf.DORMTHEME, error) {
	rows, err := queries.ListDormThemes(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	themes := make([]*protobuf.DORMTHEME, 0, len(rows))
	for _, row := range rows {
		theme := &protobuf.DORMTHEME{}
		if err := proto.Unmarshal(row.Theme, theme); err != nil {
			return nil, fmt.Errorf("dorm theme %d: %w", row.Pos, err)
		}
		theme.UploadTime = proto.Uint32(uint32(row.UploadTime))
		if row.UploadTime > 0 {
			shared, err := queries.GetSharedDormTheme(ctx, row.ThemeID)
			if err != nil {
				return nil, err
			}
			theme.LikeCount = proto.Uint32(uint32(shared.LikeCount))
			theme.FavCount = proto.Uint32(uint32(shared.FavCount))
		}
		themes = append(themes, theme)
	}
	return themes, nil
}

// sharedTheme is a shared template as the theme shop shows it.
func sharedTheme(ctx context.Context, queries *db.Queries, id string) (*protobuf.DORMTHEME, error) {
	row, err := queries.GetSharedDormTheme(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errThemeGone
	}
	if err != nil {
		return nil, err
	}
	theme := &protobuf.DORMTHEME{}
	if err := proto.Unmarshal(row.Theme, theme); err != nil {
		return nil, fmt.Errorf("dorm theme %s: %w", id, err)
	}
	theme.UploadTime = proto.Uint32(uint32(row.UploadTime))
	theme.LikeCount = proto.Uint32(uint32(row.LikeCount))
	theme.FavCount = proto.Uint32(uint32(row.FavCount))
	return theme, nil
}

// unshareTheme drops the likes and collections of a template that stops
// being shared (or is replaced, or deleted).
func unshareTheme(ctx context.Context, queries *db.Queries, id string) error {
	if err := queries.DeleteDormThemeLikes(ctx, id); err != nil {
		return err
	}
	return queries.DeleteDormThemeFavs(ctx, id)
}

// handleThemeTemplates answers BackYardGetThemeTemplateCommand for the
// commander's own templates (CS_19105, typ 2), sent when the template
// scene opens. The collection is CS_19115, the shop CS_19117.
func (game *Game) handleThemeTemplates(client *connection.Client, req *protobuf.CS_19105) error {
	reply := &protobuf.SC_19106{Result: proto.Uint32(0)}
	if req.GetTyp() == themeTemplateCustom {
		themes, err := dormThemes(context.Background(), game.DB.Queries, client.CommanderID)
		if err != nil {
			return err
		}
		reply.ThemeList = themes
	}
	return client.Send(reply)
}

// saveTheme keeps an arrangement in template slot pos (1-10), named, with
// the md5 of its preview pictures (kept by the client). A template saved
// over is no longer shared (the client builds the new one unshared).
func saveTheme(ctx context.Context, database *db.Store, commanderID, pos uint32, name string, list []*protobuf.FURNITUREPUTINFO, iconMd5, imageMd5 string) error {
	if pos < 1 || pos > dormThemeSlots {
		return errRefused
	}
	id := themeID(commanderID, pos)
	theme, err := proto.Marshal(&protobuf.DORMTHEME{
		Id:               proto.String(id),
		Name:             proto.String(name),
		FurniturePutList: list,
		UserId:           proto.Uint32(commanderID),
		Pos:              proto.Uint32(pos),
		LikeCount:        proto.Uint32(0),
		FavCount:         proto.Uint32(0),
		UploadTime:       proto.Uint32(0),
		IconImageMd5:     proto.String(iconMd5),
		ImageMd5:         proto.String(imageMd5),
	})
	if err != nil {
		return err
	}
	return database.InTx(ctx, func(queries *db.Queries) error {
		if err := unshareTheme(ctx, queries, id); err != nil {
			return err
		}
		return queries.SetDormTheme(ctx, db.SetDormThemeParams{CommanderID: commanderID, Pos: pos, Theme: theme, ThemeID: id})
	})
}

// deleteTheme empties template slot pos.
func deleteTheme(ctx context.Context, database *db.Store, commanderID, pos uint32) error {
	return database.InTx(ctx, func(queries *db.Queries) error {
		deleted, err := queries.DeleteDormTheme(ctx, db.DeleteDormThemeParams{CommanderID: commanderID, Pos: pos})
		if err != nil {
			return err
		}
		if deleted == 0 {
			return errRefused
		}
		return unshareTheme(ctx, queries, themeID(commanderID, pos))
	})
}

// handleSaveThemeTemplate answers AddBackYardThemeTemplateCommand
// (CS_19109): the arrangement saved in template slot pos. Its id is the
// commander id then pos (BackYardBaseThemeTemplate.BuildId).
func (game *Game) handleSaveThemeTemplate(client *connection.Client, req *protobuf.CS_19109) error {
	err := saveTheme(context.Background(), game.DB, client.CommanderID, req.GetPos(), req.GetName(), req.GetFurniturePutList(), req.GetIconImageMd5(), req.GetImageMd5())
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19110{Result: proto.Int32(int32(*result(err)))})
}

// handleDeleteThemeTemplate answers BackYardDeleteThemeTemplateCommand
// (CS_19123): template slot pos is emptied, gone from the shop and the
// collections if it was shared.
func (game *Game) handleDeleteThemeTemplate(client *connection.Client, req *protobuf.CS_19123) error {
	err := deleteTheme(context.Background(), game.DB, client.CommanderID, req.GetPos())
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19124{Result: proto.Int32(int32(*result(err)))})
}

// handleDormThemes answers GetDormThemeListCommand (CS_19018), the
// backyard's former themes, which the client registers but never sends
// (DormProxy lost initThemes / updateTheme): the commander's templates,
// all of them for id 0, else the one in slot id. Tested with a raw packet.
func (game *Game) handleDormThemes(client *connection.Client, req *protobuf.CS_19018) error {
	themes, err := dormThemes(context.Background(), game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	if req.GetId() != 0 {
		themes = slices.DeleteFunc(themes, func(theme *protobuf.DORMTHEME) bool { return theme.GetPos() != req.GetId() })
	}
	return client.Send(&protobuf.SC_19019{ThemeList: themes})
}

// handleSaveDormTheme answers SaveDormThemeCommand (CS_19020), the former
// themes' save, never sent by this client (the command isn't registered):
// the arrangement in template slot id, as CS_19109 without previews.
func (game *Game) handleSaveDormTheme(client *connection.Client, req *protobuf.CS_19020) error {
	err := saveTheme(context.Background(), game.DB, client.CommanderID, req.GetId(), req.GetName(), req.GetFurniturePutList(), "", "")
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19021{Result: result(err)})
}

// handleDeleteDormTheme answers BackYardDeleteThemeCommand (CS_19022), the
// former themes' delete, never sent by this client (the command isn't
// registered): template slot id is emptied, as CS_19123.
func (game *Game) handleDeleteDormTheme(client *connection.Client, req *protobuf.CS_19022) error {
	err := deleteTheme(context.Background(), game.DB, client.CommanderID, req.GetId())
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19023{Result: result(err)})
}

// handleShareThemeTemplate answers BackYardUploadThemeTemplateCommand
// (CS_19111): template slot pos goes to the theme shop, at most
// MAX_UPLOAD_THEME_CNT at a time (the client checks it too). The client
// sets the upload time itself, after uploading the previews to its OSS
// bucket.
func (game *Game) handleShareThemeTemplate(client *connection.Client, req *protobuf.CS_19111) error {
	ctx := context.Background()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		theme, err := queries.GetDormTheme(ctx, db.GetDormThemeParams{CommanderID: client.CommanderID, Pos: req.GetPos()})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		}
		if err != nil {
			return err
		}
		if theme.UploadTime > 0 {
			return errRefused
		}
		shared, err := queries.CountSharedDormThemes(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if shared >= maxSharedThemes {
			return errRefused
		}
		_, err = queries.SetDormThemeUploadTime(ctx, db.SetDormThemeUploadTimeParams{UploadTime: time.Now().Unix(), CommanderID: client.CommanderID, Pos: req.GetPos()})
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19112{Result: proto.Int32(int32(*result(err)))})
}

// handleUnshareThemeTemplate answers BackYardUnloadThemeTemplateCommand
// (CS_19125): template slot pos leaves the theme shop, with its likes, and
// the collections that held it.
func (game *Game) handleUnshareThemeTemplate(client *connection.Client, req *protobuf.CS_19125) error {
	ctx := context.Background()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		theme, err := queries.GetDormTheme(ctx, db.GetDormThemeParams{CommanderID: client.CommanderID, Pos: req.GetPos()})
		if errors.Is(err, sql.ErrNoRows) {
			return errRefused
		}
		if err != nil {
			return err
		}
		if theme.UploadTime == 0 {
			return errRefused
		}
		if _, err := queries.SetDormThemeUploadTime(ctx, db.SetDormThemeUploadTimeParams{UploadTime: 0, CommanderID: client.CommanderID, Pos: req.GetPos()}); err != nil {
			return err
		}
		return unshareTheme(ctx, queries, theme.ThemeID)
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19126{Result: proto.Int32(int32(*result(err)))})
}

// handleThemeShop answers BackYardRefreshShopTemplateCommand and the shop
// part of BackYardGetThemeTemplateCommand (CS_19117): page (from 1, at most
// DormProxy.MAX_PAGE) of num shared templates' ids, in typ's order (by
// upload time, by likes then collections, or random). Fewer than num marks
// the last page.
func (game *Game) handleThemeShop(client *connection.Client, req *protobuf.CS_19117) error {
	page, num := int(req.GetPage()), int(req.GetNum())
	if page < 1 || page > themeShopMaxPages || num < 1 {
		return client.Send(&protobuf.SC_19118{Result: proto.Int32(int32(*result(errRefused)))})
	}
	rows, err := game.DB.ListSharedDormThemes(context.Background())
	if err != nil {
		return err
	}
	hot := func(a, b db.ListSharedDormThemesRow) int {
		return cmp.Or(cmp.Compare(a.LikeCount, b.LikeCount), cmp.Compare(a.FavCount, b.FavCount), cmp.Compare(a.UploadTime, b.UploadTime))
	}
	byTime := func(a, b db.ListSharedDormThemesRow) int {
		return cmp.Or(cmp.Compare(a.UploadTime, b.UploadTime), cmp.Compare(a.ThemeID, b.ThemeID))
	}
	switch req.GetTyp() {
	case themeShopOldest:
		slices.SortFunc(rows, byTime)
	case themeShopNewest:
		slices.SortFunc(rows, func(a, b db.ListSharedDormThemesRow) int { return byTime(b, a) })
	case themeShopHottest:
		slices.SortFunc(rows, func(a, b db.ListSharedDormThemesRow) int { return hot(b, a) })
	case themeShopColdest:
		slices.SortFunc(rows, hot)
	case themeShopRandom:
		// a new draw each time: the pages aren't kept apart
		rand.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
		page = 1
	default:
		return client.Send(&protobuf.SC_19118{Result: proto.Int32(int32(*result(errRefused)))})
	}
	reply := &protobuf.SC_19118{Result: proto.Int32(0)}
	for _, row := range rows[min((page-1)*num, len(rows)):min(page*num, len(rows))] {
		reply.ThemeIdList = append(reply.ThemeIdList, row.ThemeID)
	}
	return client.Send(reply)
}

// handleThemeTemplate answers BackYardGetThemeTemplateDataCommand and
// BackYardSeachThemeTemplateCommand (CS_19113): a shared template by id,
// with whether the commander collected and liked it; result 20 when it
// isn't shared (any more).
func (game *Game) handleThemeTemplate(client *connection.Client, req *protobuf.CS_19113) error {
	ctx := context.Background()
	id := req.GetThemeId()
	theme, err := sharedTheme(ctx, game.DB.Queries, id)
	if refused(err) {
		return client.Send(&protobuf.SC_19114{Result: proto.Int32(int32(*result(err)))})
	}
	if err != nil {
		return err
	}
	faved, err := game.DB.CountDormThemeFav(ctx, db.CountDormThemeFavParams{CommanderID: client.CommanderID, ThemeID: id})
	if err != nil {
		return err
	}
	liked, err := game.DB.CountDormThemeLike(ctx, db.CountDormThemeLikeParams{ThemeID: id, CommanderID: client.CommanderID})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_19114{Result: proto.Int32(0), Theme: theme, HasFav: proto.Bool(faved > 0), HasLike: proto.Bool(liked > 0)})
}

// handleThemePreviews answers BackYardGetPreviewImageMd5Command (CS_19131):
// the icon md5 of each listed template still shared; the client drops the
// others from its shop and collection lists.
func (game *Game) handleThemePreviews(client *connection.Client, req *protobuf.CS_19131) error {
	ctx := context.Background()
	reply := &protobuf.SC_19132{}
	for _, id := range req.GetIdList() {
		theme, err := sharedTheme(ctx, game.DB.Queries, id)
		if refused(err) {
			continue
		}
		if err != nil {
			return err
		}
		reply.List = append(reply.List, &protobuf.THEME_MD5{Id: proto.String(id), Md5: proto.String(theme.GetIconImageMd5())})
	}
	return client.Send(reply)
}

// sharedThemeAt is the shared template id as shared at uploadTime (the
// client sends the time it saw): result 20 when it was taken back since.
func sharedThemeAt(ctx context.Context, queries *db.Queries, id string, uploadTime uint32) (*protobuf.DORMTHEME, error) {
	theme, err := sharedTheme(ctx, queries, id)
	if err != nil {
		return nil, err
	}
	if theme.GetUploadTime() != uploadTime {
		return nil, errThemeGone
	}
	return theme, nil
}

// handleLikeThemeTemplate answers BackYardLikeThemeTemplateCommand
// (CS_19121): one like per commander for as long as the template is
// shared, never one's own (the client hides the heart on them).
func (game *Game) handleLikeThemeTemplate(client *connection.Client, req *protobuf.CS_19121) error {
	ctx := context.Background()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		theme, err := sharedThemeAt(ctx, queries, req.GetThemeId(), req.GetUploadTime())
		if err != nil {
			return err
		}
		if theme.GetUserId() == client.CommanderID {
			return errRefused
		}
		added, err := queries.AddDormThemeLike(ctx, db.AddDormThemeLikeParams{ThemeID: req.GetThemeId(), CommanderID: client.CommanderID})
		if err != nil {
			return err
		}
		if added == 0 {
			return errRefused
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19122{Result: proto.Int32(int32(*result(err)))})
}

// handleCollectThemeTemplate answers BackYardCollectThemeTemplateCommand
// (CS_19119): a shared template joins the commander's collection, at most
// MAX_COLLECTION_CNT (the client checks it too).
func (game *Game) handleCollectThemeTemplate(client *connection.Client, req *protobuf.CS_19119) error {
	ctx := context.Background()
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		theme, err := sharedThemeAt(ctx, queries, req.GetThemeId(), req.GetUploadTime())
		if err != nil {
			return err
		}
		if theme.GetUserId() == client.CommanderID {
			return errRefused
		}
		count, err := queries.CountDormThemeFavs(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if count >= maxCollectedThemes {
			return errRefused
		}
		added, err := queries.AddDormThemeFav(ctx, db.AddDormThemeFavParams{
			CommanderID: client.CommanderID, ThemeID: req.GetThemeId(), UploadTime: int64(theme.GetUploadTime()), FavedAt: time.Now().Unix(),
		})
		if err != nil {
			return err
		}
		if added == 0 {
			return errRefused
		}
		return nil
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_19120{Result: proto.Int32(int32(*result(err)))})
}

// handleUncollectThemeTemplate answers BackYardCollectThemeTemplateCommand's
// cancel (CS_19127): the template leaves the commander's collection.
func (game *Game) handleUncollectThemeTemplate(client *connection.Client, req *protobuf.CS_19127) error {
	deleted, err := game.DB.DeleteDormThemeFav(context.Background(), db.DeleteDormThemeFavParams{CommanderID: client.CommanderID, ThemeID: req.GetThemeId()})
	if err != nil {
		return err
	}
	var refusal error
	if deleted == 0 {
		refusal = errRefused
	}
	return client.Send(&protobuf.SC_19128{Result: proto.Int32(int32(*result(refusal)))})
}

// handleCollectedThemes answers BackYardGetThemeTemplateCommand for the
// collection (CS_19115, typ 3), sent when the template scene opens: the
// collected templates still shared, with the upload time they were
// collected at.
func (game *Game) handleCollectedThemes(client *connection.Client, req *protobuf.CS_19115) error {
	rows, err := game.DB.ListDormThemeFavs(context.Background(), client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_19116{Result: proto.Int32(0)}
	for _, row := range rows {
		reply.ThemeProfileList = append(reply.ThemeProfileList, &protobuf.DORMTHEME_PROFILE{Id: proto.String(row.ThemeID), UploadTime: proto.Uint32(uint32(row.UploadTime))})
	}
	return client.Send(reply)
}

// handleReportThemeTemplate answers InformBackYardThemeTemplateCommand
// (CS_19129), reporting a shared template (the client sends it from level
// 20, reason the sum of the ticked reasons): it is only logged, like
// player reports.
func (game *Game) handleReportThemeTemplate(client *connection.Client, req *protobuf.CS_19129) error {
	ctx := context.Background()
	theme, err := sharedTheme(ctx, game.DB.Queries, req.GetThemeId())
	if err == nil && theme.GetUserId() != req.GetTargetId() {
		err = errRefused
	}
	if refused(err) {
		return client.Send(&protobuf.SC_19130{Result: proto.Int32(int32(*result(err)))})
	}
	if err != nil {
		return err
	}
	logger.WithFields("Dorm",
		logger.FieldValue("commander", client.CommanderID),
		logger.FieldValue("reported", req.GetTargetId()),
		logger.FieldValue("theme", req.GetThemeId()),
		logger.FieldValue("name", req.GetThemeName()),
		logger.FieldValue("reason", req.GetReason()),
	).Info("backyard theme reported")
	return client.Send(&protobuf.SC_19130{Result: proto.Int32(0)})
}

// handleThemeStorage answers GetOSSArgsCommand (CS_19103): temporary
// credentials to the OSS bucket the backyard previews are uploaded to,
// asked only by the CN client (OSSMgr has no instance elsewhere). Belfast
// has no bucket: refused.
//
//belfast:static
func (game *Game) handleThemeStorage(client *connection.Client, req *protobuf.CS_19103) error {
	return client.Send(&protobuf.SC_19104{Result: result(errRefused)})
}

// handleVisitBackyard answers VisitBackYardCommand (CS_19101), from a
// friend's profile: the friend's backyard as SC_19001 shows one's own, lv 0
// (backyard_unopen) below the commander level the backyard opens at. A
// first floor never arranged shows the starter arrangement, as CS_19024
// does for one's own.
func (game *Game) handleVisitBackyard(client *connection.Client, req *protobuf.CS_19101) error {
	ctx := context.Background()
	friendID := req.GetUserId()
	friends, err := game.DB.CountFriend(ctx, db.CountFriendParams{CommanderID: client.CommanderID, FriendID: friendID})
	if err != nil {
		return err
	}
	friend, err := commanderByID(ctx, game.DB.Queries, friendID)
	if err != nil && !refused(err) {
		return err
	}
	if friends == 0 || err != nil || friend.Level < dormOpenLevel {
		return client.Send(&protobuf.SC_19102{Lv: proto.Uint32(0)})
	}
	inDorm, err := game.DB.ListDormShips(ctx, friendID)
	if err != nil {
		return err
	}
	ids := make([]uint32, len(inDorm))
	for i, ship := range inDorm {
		ids[i] = ship.ShipID
	}
	ships, err := game.DB.ListShipsByID(ctx, db.ListShipsByIDParams{CommanderID: friendID, Ids: ids})
	if err != nil {
		return err
	}
	dorm, err := game.dorm(ctx, game.DB.Queries, friend, ships)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(dorm.FurniturePutList, func(floor *protobuf.FURFLOORPUTINFO) bool { return floor.GetFloor() == dormFloorTrain }) {
		dorm.FurniturePutList = append([]*protobuf.FURFLOORPUTINFO{{Floor: proto.Uint32(dormFloorTrain), FurniturePutList: furnitureStart}}, dorm.FurniturePutList...)
	}
	return client.Send(&protobuf.SC_19102{
		Lv:                   dorm.Lv,
		Food:                 dorm.Food,
		FoodMaxIncrease:      dorm.FoodMaxIncrease,
		FoodMaxIncreaseCount: dorm.FoodMaxIncreaseCount,
		ShipList:             dorm.ShipList,
		FurnitureIdList:      dorm.FurnitureIdList,
		FloorNum:             dorm.FloorNum,
		ExpPos:               dorm.ExpPos,
		FurniturePutList:     dorm.FurniturePutList,
		Name:                 dorm.Name,
	})
}
