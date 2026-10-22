package admin

import (
	"net/http"
	"sort"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// registerModeration registers the API of reports, auto-flags, shared content and guilds.
func (server *Server) registerModeration(api apiFunc) {
	api("GET /api/moderation/reports", server.handleReports)
	api("POST /api/moderation/reports/{kind}/{id}/dismiss", server.handleDismissReports)
	api("DELETE /api/moderation/comments/{id}", server.handleDeleteComment)
	api("DELETE /api/moderation/gear-codes/{id}", server.handleDeleteGearCode)
	api("GET /api/moderation/flags", server.handleFlags)
	api("DELETE /api/moderation/flags/{id}", server.handleDismissFlags)
	api("GET /api/moderation/shared", server.handleShared)
	api("POST /api/moderation/themes/{id}/{pos}/unshare", server.handleUnshareTheme)
	server.registerGuilds(api)
}

// shipRef is the ship group a comment or a gear code is about.
type shipRef struct {
	Group  uint32 `json:"group"`
	Name   string `json:"name"`
	Rarity uint32 `json:"rarity"`
	Icon   string `json:"icon,omitempty"`
}

func (tables *moderationTables) shipRef(group uint32) shipRef {
	row := tables.ship(group)
	return shipRef{Group: group, Name: row.Name, Rarity: row.Rarity, Icon: row.Icon}
}

type reportLine struct {
	CommanderID uint32 `json:"commander_id"`
	Name        string `json:"name"`
	Reason      string `json:"reason"`
	At          int64  `json:"at"`
	// the reporter's avatar, its flagship's icon
	Icon   string `json:"icon,omitempty"`
	Rarity uint32 `json:"rarity,omitempty"`
}

// reportedContent is a comment or a gear code with its reports.
type reportedContent struct {
	Kind     string  `json:"kind"` // comment, gear
	ID       uint32  `json:"id"`
	Ship     shipRef `json:"ship"`
	AuthorID uint32  `json:"author_id"`
	Author   string  `json:"author"`
	// the author's avatar, its flagship's icon
	AuthorIcon   string       `json:"author_icon,omitempty"`
	AuthorRarity uint32       `json:"author_rarity,omitempty"`
	Content      string       `json:"content"`
	PostedAt     int64        `json:"posted_at"`
	Reports      []reportLine `json:"reports"`
}

// equipReportReason is a CS_17607 report_type as the impeach panel words
// it (equipcode_report_type_<n>).
func (tables *moderationTables) equipReportReason(kind uint32) string {
	if tip := tables.tips["equipcode_report_type_"+itoa(kind)]; tip != "" {
		return tip
	}
	return "Type " + itoa(kind)
}

// handleReports is the report queue: every reported ship comment and gear
// code with its reporters, the most reported first.
func (server *Server) handleReports(r *http.Request) (any, error) {
	tables, err := server.tables()
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	icons, err := server.avatars(ctx)
	if err != nil {
		return nil, err
	}
	comments, err := server.Store.ModerationListDiscussionReports(ctx)
	if err != nil {
		return nil, err
	}
	codes, err := server.Store.ModerationListEquipCodeReports(ctx)
	if err != nil {
		return nil, err
	}
	byKey := map[string]*reportedContent{}
	list := []*reportedContent{}
	add := func(kind string, id uint32, fill func() reportedContent, line reportLine) {
		key := kind + itoa(id)
		item, ok := byKey[key]
		if !ok {
			content := fill()
			item = &content
			byKey[key] = item
			list = append(list, item)
		}
		item.Reports = append(item.Reports, line)
	}
	for _, row := range comments {
		add("comment", row.DiscussionID, func() reportedContent {
			return reportedContent{Kind: "comment", ID: row.DiscussionID, Ship: tables.shipRef(row.GroupID), AuthorID: row.AuthorID, Author: row.Author, AuthorIcon: icons[row.AuthorID].Icon, AuthorRarity: icons[row.AuthorID].Rarity, Content: row.Context, PostedAt: row.PostedAt}
		}, reportLine{CommanderID: row.CommanderID, Name: row.Reporter, Reason: row.Reason, At: row.ReportedAt, Icon: icons[row.CommanderID].Icon, Rarity: icons[row.CommanderID].Rarity})
	}
	for _, row := range codes {
		add("gear", row.ShareID, func() reportedContent {
			return reportedContent{Kind: "gear", ID: row.ShareID, Ship: tables.shipRef(row.GroupID), AuthorID: row.AuthorID, Author: row.Author, AuthorIcon: icons[row.AuthorID].Icon, AuthorRarity: icons[row.AuthorID].Rarity, Content: row.Eqcode, PostedAt: row.SharedAt}
		}, reportLine{CommanderID: row.CommanderID, Name: row.Reporter, Reason: tables.equipReportReason(row.ReportType), At: row.ReportedAt, Icon: icons[row.CommanderID].Icon, Rarity: icons[row.CommanderID].Rarity})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if len(list[i].Reports) != len(list[j].Reports) {
			return len(list[i].Reports) > len(list[j].Reports)
		}
		return list[i].Reports[0].At > list[j].Reports[0].At
	})
	return list, nil
}

// handleDismissReports drops the reports of a comment or a gear code.
func (server *Server) handleDismissReports(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var deleted int64
	switch r.PathValue("kind") {
	case "comment":
		deleted, err = server.Store.ModerationDeleteDiscussionReports(r.Context(), id)
	case "gear":
		deleted, err = server.Store.ModerationDeleteEquipCodeReports(r.Context(), id)
	default:
		return nil, badRequest("kind: comment or gear")
	}
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return nil, notFound("no reports of it")
	}
	return map[string]int64{"dismissed": deleted}, nil
}

// handleDeleteComment deletes a ship comment with its reviews and reports.
// Clients fetch a ship's comments each time its page opens
// (FetchEvaluationCommand), so no one is kicked.
func (server *Server) handleDeleteComment(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	err = server.Store.InTx(ctx, func(queries *db.Queries) error {
		if _, err := queries.ModerationDeleteDiscussionReports(ctx, id); err != nil {
			return err
		}
		if err := queries.ModerationDeleteDiscussionVotes(ctx, id); err != nil {
			return err
		}
		deleted, err := queries.ModerationDeleteDiscussion(ctx, id)
		if err == nil && deleted == 0 {
			return notFound("no comment " + itoa(id))
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// handleDeleteGearCode deletes a shared gear code with its likes and
// reports. Clients fetch a ship's codes each time its page opens
// (EquipCodeShareLayer, CS_17601), so no one is kicked.
func (server *Server) handleDeleteGearCode(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	err = server.Store.InTx(ctx, func(queries *db.Queries) error {
		if _, err := queries.ModerationDeleteEquipCodeReports(ctx, id); err != nil {
			return err
		}
		if err := queries.ModerationDeleteEquipCodeLikes(ctx, id); err != nil {
			return err
		}
		deleted, err := queries.ModerationDeleteEquipCode(ctx, id)
		if err == nil && deleted == 0 {
			return notFound("no gear code " + itoa(id))
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// CS_10994 reasons (CheaterMarkCommand): a battle's verification type
// (BattleState.Vertify, CC_TYPE_*), 900 + it in an exercise
// (BattleDuelArenaCommand), and the CN client's switches (HXSet).
const (
	markLocalization = 99  // CC_TYPE_99, HXSet codeMode
	markAntiSkin     = 100 // CC_TYPE_100, HXSet antiSkinMode
	markExercise     = 900
)

// markKind sorts a reason: battle (a check failed) or client (a switch).
func markKind(reason uint32) string {
	if reason == markLocalization || reason == markAntiSkin {
		return "client"
	}
	return "battle"
}

func markLabel(reason uint32) string {
	switch {
	case reason == markLocalization:
		return "Localization"
	case reason == markAntiSkin:
		return "Anti-skin"
	case reason >= markExercise:
		return "Exercise check"
	}
	return "Battle check"
}

// markDetail is a reason as one line says it.
func markDetail(reason uint32) string {
	switch {
	case reason == markLocalization:
		return "Localization switch on"
	case reason == markAntiSkin:
		return "Anti-skin switch on"
	case reason >= markExercise:
		return "Exercise check failed, type " + itoa(reason-markExercise)
	}
	return "Battle check failed, type " + itoa(reason)
}

type markRow struct {
	ID     uint32 `json:"id"`
	Reason uint32 `json:"reason"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	At     int64  `json:"at"`
}

type markTally struct {
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

type flaggedCommander struct {
	ID        uint32 `json:"id"`
	Name      string `json:"name"`
	AccountID uint32 `json:"account_id"`
	// its avatar, its flagship's icon
	Icon    string      `json:"icon,omitempty"`
	Rarity  uint32      `json:"rarity,omitempty"`
	Online  bool        `json:"online"`
	First   int64       `json:"first"`
	Last    int64       `json:"last"`
	Tallies []markTally `json:"tallies"`
	Marks   []markRow   `json:"marks"`
}

// handleFlags lists the cheater marks per commander, the most marked first,
// with the day's and the week's counts.
func (server *Server) handleFlags(r *http.Request) (any, error) {
	marks, err := server.Store.ModerationListCheaterMarks(r.Context())
	if err != nil {
		return nil, err
	}
	icons, err := server.avatars(r.Context())
	if err != nil {
		return nil, err
	}
	now := time.Now().In(gamedata.Zone())
	week, _ := game.GuildRankPeriods(now)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	reply := struct {
		Today      int                 `json:"today"`
		Week       int                 `json:"week"`
		Commanders []*flaggedCommander `json:"commanders"`
	}{Commanders: []*flaggedCommander{}}
	byID := map[uint32]*flaggedCommander{}
	for _, mark := range marks {
		if mark.MarkedAt >= today {
			reply.Today++
		}
		if mark.MarkedAt >= int64(week) {
			reply.Week++
		}
		flagged, ok := byID[mark.CommanderID]
		if !ok {
			flagged = &flaggedCommander{ID: mark.CommanderID, Name: mark.Name, AccountID: mark.AccountID, Icon: icons[mark.CommanderID].Icon, Rarity: icons[mark.CommanderID].Rarity, Online: server.Game.IsOnline(mark.CommanderID), Last: mark.MarkedAt}
			byID[mark.CommanderID] = flagged
			reply.Commanders = append(reply.Commanders, flagged)
		}
		flagged.First = mark.MarkedAt
		flagged.Marks = append(flagged.Marks, markRow{ID: mark.ID, Reason: mark.Reason, Kind: markKind(mark.Reason), Detail: markDetail(mark.Reason), At: mark.MarkedAt})
		label := markLabel(mark.Reason)
		found := false
		for i := range flagged.Tallies {
			if flagged.Tallies[i].Label == label {
				flagged.Tallies[i].Count++
				found = true
			}
		}
		if !found {
			flagged.Tallies = append(flagged.Tallies, markTally{Label: label, Kind: markKind(mark.Reason), Count: 1})
		}
	}
	sort.SliceStable(reply.Commanders, func(i, j int) bool {
		a, b := reply.Commanders[i], reply.Commanders[j]
		if len(a.Marks) != len(b.Marks) {
			return len(a.Marks) > len(b.Marks)
		}
		return a.Last > b.Last
	})
	return reply, nil
}

// handleDismissFlags drops a commander's cheater marks.
func (server *Server) handleDismissFlags(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	deleted, err := server.Store.ModerationDismissCheaterMarks(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if deleted == 0 {
		return nil, notFound("no marks for commander " + itoa(id))
	}
	return map[string]int64{"dismissed": deleted}, nil
}

type sharedComment struct {
	ID           uint32       `json:"id"`
	Ship         shipRef      `json:"ship"`
	AuthorID     uint32       `json:"author_id"`
	Author       string       `json:"author"`
	AuthorIcon   string       `json:"author_icon,omitempty"`
	AuthorRarity uint32       `json:"author_rarity,omitempty"`
	Content      string       `json:"content"`
	PostedAt     int64        `json:"posted_at"`
	Good         int64        `json:"good"`
	Bad          int64        `json:"bad"`
	Reports      []reportLine `json:"reports"`
}

type sharedCode struct {
	ID           uint32  `json:"id"`
	Ship         shipRef `json:"ship"`
	AuthorID     uint32  `json:"author_id"`
	Author       string  `json:"author"`
	AuthorIcon   string  `json:"author_icon,omitempty"`
	AuthorRarity uint32  `json:"author_rarity,omitempty"`
	Code         string  `json:"code"`
	SharedAt     int64   `json:"shared_at"`
	Likes        uint32  `json:"likes"`
	Nonsense     int64   `json:"nonsense"`
	Outdated     int64   `json:"outdated"`
}

type sharedTheme struct {
	AuthorID     uint32 `json:"author_id"`
	Author       string `json:"author"`
	AuthorIcon   string `json:"author_icon,omitempty"`
	AuthorRarity uint32 `json:"author_rarity,omitempty"`
	Pos          uint32 `json:"pos"`
	Name         string `json:"name"`
	SharedAt     int64  `json:"shared_at"`
	Likes        int64  `json:"likes"`
	Favs         int64  `json:"favs"`
}

// handleShared lists what commanders share with the whole server: ship
// comments (with their reviews and reports), gear codes and backyard
// templates in the theme shop.
func (server *Server) handleShared(r *http.Request) (any, error) {
	tables, err := server.tables()
	if err != nil {
		return nil, err
	}
	ctx := r.Context()
	reply := struct {
		Comments []*sharedComment `json:"comments"`
		Codes    []sharedCode     `json:"codes"`
		Themes   []sharedTheme    `json:"themes"`
	}{Comments: []*sharedComment{}, Codes: []sharedCode{}, Themes: []sharedTheme{}}
	icons, err := server.avatars(ctx)
	if err != nil {
		return nil, err
	}
	comments, err := server.Store.ModerationListDiscussions(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[uint32]*sharedComment{}
	for _, row := range comments {
		comment := &sharedComment{
			ID: row.ID, Ship: tables.shipRef(row.GroupID), AuthorID: row.CommanderID, Author: row.Author, AuthorIcon: icons[row.CommanderID].Icon, AuthorRarity: icons[row.CommanderID].Rarity,
			Content: row.Context, PostedAt: row.PostedAt, Good: row.GoodCount, Bad: row.BadCount, Reports: []reportLine{},
		}
		byID[row.ID] = comment
		reply.Comments = append(reply.Comments, comment)
	}
	reports, err := server.Store.ModerationListDiscussionReports(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range reports {
		if comment := byID[row.DiscussionID]; comment != nil {
			comment.Reports = append(comment.Reports, reportLine{CommanderID: row.CommanderID, Name: row.Reporter, Reason: row.Reason, At: row.ReportedAt, Icon: icons[row.CommanderID].Icon, Rarity: icons[row.CommanderID].Rarity})
		}
	}
	codes, err := server.Store.ModerationListEquipCodes(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range codes {
		reply.Codes = append(reply.Codes, sharedCode{
			ID: row.ID, Ship: tables.shipRef(row.GroupID), AuthorID: row.CommanderID, Author: row.Author, AuthorIcon: icons[row.CommanderID].Icon, AuthorRarity: icons[row.CommanderID].Rarity, Code: row.Eqcode,
			SharedAt: row.SharedAt, Likes: row.Likes, Nonsense: row.Nonsense, Outdated: row.Outdated,
		})
	}
	themes, err := server.Store.ModerationListSharedDormThemes(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range themes {
		theme := &protobuf.DORMTHEME{}
		name := ""
		if proto.Unmarshal(row.Theme, theme) == nil {
			name = theme.GetName()
		}
		reply.Themes = append(reply.Themes, sharedTheme{
			AuthorID: row.CommanderID, Author: row.Author, AuthorIcon: icons[row.CommanderID].Icon, AuthorRarity: icons[row.CommanderID].Rarity, Pos: row.Pos, Name: name, SharedAt: row.UploadTime, Likes: row.LikeCount, Favs: row.FavCount,
		})
	}
	return reply, nil
}

// handleUnshareTheme takes a template out of the theme shop.
func (server *Server) handleUnshareTheme(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	pos, err := pathUint(r, "pos")
	if err != nil {
		return nil, err
	}
	return kicked(server.Game.UnshareDormTheme(r.Context(), id, pos))
}
