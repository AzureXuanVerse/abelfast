package admin

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/game"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/region"
)

// registerContent registers the API of mail everyone, redeem codes, events and server-wide data.
func (server *Server) registerContent(api apiFunc) {
	api("PUT /api/notices/{id}", server.handleUpdateNotice)
	api("POST /api/broadcast/audience", server.handleAudience)
	api("POST /api/broadcast", server.handleBroadcast)
	api("GET /api/codes", server.handleCodes)
	api("POST /api/codes", server.handleCreateCode)
	api("POST /api/codes/{code}/end", server.handleEndCode)
	api("DELETE /api/codes/{code}", server.handleDeleteCode)
	api("GET /api/events", server.handleEvents)
	api("GET /api/shared", server.handleSharedData)
	api("POST /api/shared/marks/reset", server.handleResetMark)
	api("POST /api/shared/surveys/{id}/reset", server.handleResetSurvey)
}

// serverClockTime reads a "2006-01-02T15:04" (or a date alone) time of the server
// clock, as the web app's date inputs give it.
func serverClockTime(field, value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02"} {
		if at, err := time.ParseInLocation(layout, value, gamedata.Zone()); err == nil {
			return at, nil
		}
	}
	return time.Time{}, badRequest(field + ": want YYYY-MM-DDTHH:MM")
}

// handleUpdateNotice changes a notice, raising its version so clients
// fetch its images again; they see it at their next login.
func (server *Server) handleUpdateNotice(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	notice, err := server.Store.GetNotice(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("no notice " + itoa(id))
	}
	if err != nil {
		return nil, err
	}
	body := noticeRow{
		Kind: notice.Kind, Button: notice.BtnTitle, Title: notice.Title, TitleImage: notice.TitleImage, TimeDesc: notice.TimeDesc,
		Content: notice.Content, Tag: notice.TagType, Icon: notice.Icon, Priority: notice.Priority, NeedLevel: notice.NeedLevel,
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	notice.Kind, notice.BtnTitle, notice.Title, notice.TitleImage, notice.TimeDesc = body.Kind, body.Button, body.Title, body.TitleImage, body.TimeDesc
	notice.Content, notice.TagType, notice.Icon, notice.Priority, notice.NeedLevel = body.Content, body.Tag, body.Icon, body.Priority, body.NeedLevel
	if notice.BtnTitle == "" {
		notice.BtnTitle = notice.Title
	}
	notice.Version++
	if err := game.CheckNotice(notice); err != nil {
		return nil, badRequest(err.Error())
	}
	params := game.NoticeParams(notice)
	err = server.Store.UpdateNotice(r.Context(), db.UpdateNoticeParams{
		Kind: params.Kind, Version: params.Version, BtnTitle: params.BtnTitle, Title: params.Title,
		TitleImage: params.TitleImage, TimeDesc: params.TimeDesc, Content: params.Content,
		TagType: params.TagType, Icon: params.Icon, Track: params.Track, Priority: params.Priority,
		NeedLevel: params.NeedLevel, ID: notice.ID,
	})
	if err != nil {
		return nil, err
	}
	return map[string]uint32{"id": notice.ID, "version": notice.Version}, nil
}

// audience is who a broadcast reaches: everyone, the commanders playing,
// a level range, the ones registered before a day (server clock), or the
// ones picked.
type audience struct {
	Kind             string   `json:"kind"`
	LevelMin         uint32   `json:"level_min"`
	LevelMax         uint32   `json:"level_max"`
	RegisteredBefore string   `json:"registered_before"`
	IDs              []uint32 `json:"ids"`
}

// resolve lists the ids of the commanders the audience reaches.
func (server *Server) resolve(r *http.Request, who audience) ([]uint32, error) {
	commanders, err := server.Store.ListCommanders(r.Context())
	if err != nil {
		return nil, err
	}
	var keep func(db.ListCommandersRow) bool
	switch who.Kind {
	case "everyone":
		keep = func(db.ListCommandersRow) bool { return true }
	case "online":
		keep = func(c db.ListCommandersRow) bool { return server.Game.IsOnline(c.ID) }
	case "level":
		if who.LevelMax != 0 && who.LevelMax < who.LevelMin {
			return nil, badRequest("level_max under level_min")
		}
		keep = func(c db.ListCommandersRow) bool {
			return c.Level >= who.LevelMin && (who.LevelMax == 0 || c.Level <= who.LevelMax)
		}
	case "registered":
		before, err := serverClockTime("registered_before", who.RegisteredBefore)
		if err != nil {
			return nil, err
		}
		keep = func(c db.ListCommandersRow) bool { return c.RegisteredAt < before.Unix() }
	case "picked":
		known := map[uint32]bool{}
		for _, c := range commanders {
			known[c.ID] = true
		}
		for _, id := range who.IDs {
			if !known[id] {
				return nil, badRequest("no commander " + itoa(id))
			}
		}
		keep = func(c db.ListCommandersRow) bool { return slices.Contains(who.IDs, c.ID) }
	default:
		return nil, badRequest("kind: want everyone, online, level, registered or picked")
	}
	ids := []uint32{}
	for _, c := range commanders {
		if keep(c) {
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

// handleAudience counts the commanders a broadcast would reach.
func (server *Server) handleAudience(r *http.Request) (any, error) {
	var who audience
	if err := readJSON(r, &who); err != nil {
		return nil, err
	}
	ids, err := server.resolve(r, who)
	if err != nil {
		return nil, err
	}
	online := 0
	for _, id := range ids {
		if server.Game.IsOnline(id) {
			online++
		}
	}
	return map[string]int{"count": len(ids), "online": online}, nil
}

// handleBroadcast mails an audience, or sends it a compensation when
// expire_hours is set.
func (server *Server) handleBroadcast(r *http.Request) (any, error) {
	var body struct {
		Audience    audience `json:"audience"`
		Title       string   `json:"title"`
		Sender      string   `json:"sender"`
		Content     string   `json:"content"`
		Attachments []drop   `json:"attachments"`
		ExpireHours uint32   `json:"expire_hours"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	ids, err := server.resolve(r, body.Audience)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, badRequest("no commander to send to")
	}
	attachments, err := dropInfos(body.Attachments)
	if err != nil {
		return nil, err
	}
	var expires time.Time
	kind := "mail"
	if body.ExpireHours != 0 {
		expires, kind = time.Now().Add(time.Duration(body.ExpireHours)*time.Hour), "compensation"
	}
	mail := game.Mail{Title: body.Title, Sender: body.Sender, Content: body.Content, Attachments: attachments}
	sent, err := server.Game.Broadcast(r.Context(), ids, mail, expires)
	if err != nil && sent == 0 {
		return nil, badRequest(err.Error())
	}
	return map[string]any{"sent": sent, "kind": kind}, err
}

type codeRow struct {
	Code    string   `json:"code"`
	Title   string   `json:"title"`
	Sender  string   `json:"sender"`
	Content string   `json:"content"`
	Rewards []reward `json:"rewards"`
	// unix seconds, 0 without a window
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
	Status string `json:"status"` // active, scheduled, ended
	Uses   int64  `json:"uses"`
}

type redemptionRow struct {
	At          int64  `json:"at"`
	CommanderID uint32 `json:"commander_id"`
	Name        string `json:"name"`
	// the commander's avatar, its flagship's icon
	Icon   string `json:"icon,omitempty"`
	Rarity uint32 `json:"rarity,omitempty"`
	Code   string `json:"code"`
	// the code's rewards now, none once it was deleted
	Rewards []reward `json:"rewards"`
}

// reward is a drop with its name and picture.
type reward struct {
	Type   uint32 `json:"type"`
	ID     uint32 `json:"id"`
	Count  uint32 `json:"count"`
	Name   string `json:"name,omitempty"`
	Rarity uint32 `json:"rarity,omitempty"`
	Icon   string `json:"icon,omitempty"`
}

// namedDrops names a list of [type, id, count].
func (server *Server) namedDrops(list [][3]uint32) []reward {
	drops := make([]reward, 0, len(list))
	for _, d := range list {
		row := server.names.shown(d[0], d[1])
		drops = append(drops, reward{Type: d[0], ID: d[1], Count: d[2], Name: row.Name, Rarity: row.Rarity, Icon: row.Icon})
	}
	return drops
}

// handleCodes lists the redeem codes and the latest redemptions.
func (server *Server) handleCodes(r *http.Request) (any, error) {
	uses, err := server.Store.CountExchangeCodeUses(r.Context())
	if err != nil {
		return nil, err
	}
	used := map[string]int64{}
	for _, row := range uses {
		used[row.Code] = row.Uses
	}
	now := time.Now()
	codes := []codeRow{}
	rewards := map[string][]reward{}
	for _, code := range game.ExchangeCodes() {
		row := codeRow{Code: code.Code, Title: code.Title, Sender: code.Sender, Content: code.Content, Rewards: server.namedDrops(code.Rewards), Status: "active", Uses: used[code.Code]}
		if !code.Always {
			row.Start, row.End = code.Start.Unix(), code.End.Unix()
			switch {
			case code.Start.IsZero() || !now.Before(code.End):
				row.Status = "ended"
			case now.Before(code.Start):
				row.Status = "scheduled"
			}
			if code.Start.IsZero() {
				row.Start, row.End = 0, 0
			}
		}
		rewards[code.Code] = row.Rewards
		codes = append(codes, row)
	}
	redeemed, err := server.Store.ListExchangeCodeRedemptions(r.Context(), 200)
	if err != nil {
		return nil, err
	}
	icons, err := server.avatars(r.Context())
	if err != nil {
		return nil, err
	}
	log := make([]redemptionRow, 0, len(redeemed))
	for _, row := range redeemed {
		avatar := icons[row.CommanderID]
		row := redemptionRow{At: row.RedeemedAt, CommanderID: row.CommanderID, Name: row.Name, Icon: avatar.Icon, Rarity: avatar.Rarity, Code: row.Code, Rewards: rewards[row.Code]}
		if row.Rewards == nil {
			row.Rewards = []reward{}
		}
		log = append(log, row)
	}
	return map[string]any{"codes": codes, "redemptions": log, "offset": game.Clock(now).Offset}, nil
}

// handleCreateCode adds a redeem code to server/exchange_codes.json; it
// works at once.
func (server *Server) handleCreateCode(r *http.Request) (any, error) {
	var body struct {
		Code    string `json:"code"`
		Title   string `json:"title"`
		Sender  string `json:"sender"`
		Content string `json:"content"`
		Rewards []drop `json:"rewards"`
		// server clock, both empty for always
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	code := game.ExchangeCode{Code: body.Code, Title: body.Title, Sender: body.Sender, Content: body.Content, Always: body.Start == "" && body.End == ""}
	for _, d := range body.Rewards {
		if d.Count == 0 {
			return nil, badRequest("a reward of 0")
		}
		code.Rewards = append(code.Rewards, [3]uint32{d.Type, d.ID, d.Count})
	}
	if !code.Always {
		code.Start = time.Now()
		if body.Start != "" {
			start, err := serverClockTime("start", body.Start)
			if err != nil {
				return nil, err
			}
			code.Start = start
		}
		if body.End == "" {
			return nil, badRequest("end: a code that starts needs an end")
		}
		end, err := serverClockTime("end", body.End)
		if err != nil {
			return nil, err
		}
		code.End = end
	}
	if err := game.CreateExchangeCode(code); err != nil {
		return nil, badRequest(err.Error())
	}
	return map[string]string{"code": strings.ToUpper(strings.TrimSpace(body.Code))}, nil
}

// exchangeCodeError answers ErrNoCode with a 404.
func exchangeCodeError(r *http.Request, err error) (any, error) {
	if errors.Is(err, game.ErrNoCode) {
		return nil, notFound("no code " + r.PathValue("code"))
	}
	if err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

// handleEndCode ends a code's window now.
func (server *Server) handleEndCode(r *http.Request) (any, error) {
	return exchangeCodeError(r, game.EndExchangeCode(r.PathValue("code"), time.Now()))
}

// handleDeleteCode removes a code; its redemptions stay.
func (server *Server) handleDeleteCode(r *http.Request) (any, error) {
	return exchangeCodeError(r, game.DeleteExchangeCode(r.PathValue("code")))
}

// eventActivityRow is an activity_template row as the events screen shows it:
// its type, a name from the tables when they have one, and its timer
// window (unix seconds).
type eventActivityRow struct {
	ID     uint32 `json:"id"`
	Type   uint32 `json:"type"`
	Name   string `json:"name"`
	Shown  bool   `json:"shown"`
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
	Status string `json:"status"` // live, upcoming, ended
}

// eventActivityTemplate is what the events screen reads of activity_template.
type eventActivityTemplate struct {
	Type        uint32          `json:"type"`
	Time        gamedata.Window `json:"time"`
	IsShow      uint32          `json:"is_show"`
	TitleResTag string          `json:"title_res_tag"`
	PageInfo    json.RawMessage `json:"page_info"`
}

// contentActivities is activity_template with each row's name: its activity_const
// key, else its page's class, else its title image.
var contentActivities = sync.OnceValues(func() (map[uint32]eventActivityRow, error) {
	rows, err := gamedata.Table[eventActivityTemplate]("activity_template")
	if err != nil {
		return nil, err
	}
	names := map[uint32]string{}
	for _, sub := range []string{"sharecfgdata", "ShareCfg"} {
		raw, err := os.ReadFile(filepath.Join(gamedata.RegionDir(), sub, "activity_const.json"))
		if err != nil {
			continue
		}
		var consts map[string]struct {
			ActID uint32 `json:"act_id"`
		}
		if json.Unmarshal(raw, &consts) == nil {
			for name, row := range consts {
				if row.ActID != 0 && (names[row.ActID] == "" || name < names[row.ActID]) {
					names[row.ActID] = name
				}
			}
			break
		}
	}
	byID := make(map[uint32]eventActivityRow, len(rows))
	for id, row := range rows {
		if row.Time.Start.IsZero() || row.Time.NewUser != nil {
			continue
		}
		var page struct {
			ClassName string `json:"class_name"`
		}
		json.Unmarshal(row.PageInfo, &page)
		byID[id] = eventActivityRow{
			ID: id, Type: row.Type, Name: cmp.Or(names[id], page.ClassName, row.TitleResTag), Shown: row.IsShow > 0,
			Start: row.Time.Start.Unix(), End: row.Time.End.Unix(),
		}
	}
	return byID, nil
})

// eventsWindow is how far back the events screen lists ended activities.
const eventsWindow = 60 * 24 * time.Hour

type shopRotationRow struct {
	Name string `json:"name"`
	Rule string `json:"rule"`
	Next int64  `json:"next"`
}

type buildPoolRow struct {
	ID    uint32             `json:"id"`
	Name  string             `json:"name"`
	Rates map[string]float64 `json:"rates"` // by ship rarity
	Ships int                `json:"ships"`
}

// handleEvents is the server's schedule: the activities with a timer
// running, coming or ended lately (live is what openActivities runs:
// activity_template's window holds now), when the shops refresh, the
// construction rates and the server clock.
func (server *Server) handleEvents(r *http.Request) (any, error) {
	rows, err := contentActivities()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	list := []eventActivityRow{}
	for _, row := range rows {
		switch {
		case now.Unix() >= row.End:
			if now.Unix()-row.End > int64(eventsWindow/time.Second) {
				continue
			}
			row.Status = "ended"
		case now.Unix() < row.Start:
			row.Status = "upcoming"
		default:
			row.Status = "live"
		}
		list = append(list, row)
	}
	slices.SortFunc(list, func(a, b eventActivityRow) int {
		return cmp.Or(cmp.Compare(b.End, a.End), cmp.Compare(a.Start, b.Start), cmp.Compare(a.ID, b.ID))
	})
	rotations := []shopRotationRow{}
	for _, rotation := range game.ShopRotations(now) {
		rotations = append(rotations, shopRotationRow{rotation.Name, rotation.Rule, rotation.Next.Unix()})
	}
	pools := []buildPoolRow{}
	for _, pool := range game.BuildPools() {
		row := buildPoolRow{ID: pool.ID, Name: pool.Name, Rates: map[string]float64{}, Ships: pool.Ships}
		for rarity, rate := range pool.Rates {
			row.Rates[strconv.FormatUint(uint64(rarity), 10)] = rate
		}
		pools = append(pools, row)
	}
	clock := game.Clock(now)
	return map[string]any{
		"region": region.Current(), "data": filepath.Base(gamedata.Dir()),
		"activities": list, "shops": rotations, "pools": pools,
		"clock": map[string]any{
			"now": clock.Now.Unix(), "zone": clock.Zone, "offset": clock.Offset,
			"next_day": clock.NextDay.Unix(), "next_week": clock.NextWeek.Unix(), "next_month": clock.NextMonth.Unix(),
		},
	}, nil
}

type activityBossRow struct {
	ActivityID uint32        `json:"activity_id"`
	BossID     uint32        `json:"boss_id"`
	Collab     bool          `json:"collab"`
	Series     uint32        `json:"series"`
	Name       string        `json:"name"`
	Open       bool          `json:"open"`
	HP         uint32        `json:"hp"` // left, in 1/10000
	Marks      []bossMarkRow `json:"marks"`
}

type bossMarkRow struct {
	Mark      uint32 `json:"mark"`
	ReachedAt int64  `json:"reached_at"` // 0 not yet
}

type surveyTakenRow struct {
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Open   bool   `json:"open"`
	Taken  int64  `json:"taken"`
	Latest int64  `json:"latest"`
}

// handleSharedData is the data every commander shares: the activity bosses'
// HP and marks, and the surveys taken.
func (server *Server) handleSharedData(r *http.Request) (any, error) {
	now := time.Now()
	bosses, err := server.Game.ActivityBosses(r.Context(), now)
	if err != nil {
		return nil, err
	}
	bossRows := []activityBossRow{}
	for _, boss := range bosses {
		row := activityBossRow{ActivityID: boss.ActivityID, BossID: boss.BossID, Collab: boss.Collab, Series: boss.Series, Name: boss.Name, Open: boss.Open, HP: boss.HP}
		for _, mark := range boss.Marks {
			row.Marks = append(row.Marks, bossMarkRow{mark, boss.Reached[mark]})
		}
		bossRows = append(bossRows, row)
	}
	surveys, err := server.Game.Surveys(r.Context(), now)
	if err != nil {
		return nil, err
	}
	surveyRows := []surveyTakenRow{}
	for _, survey := range surveys {
		surveyRows = append(surveyRows, surveyTakenRow{survey.ID, survey.Name, survey.Open, survey.Taken, survey.Latest})
	}
	commanders, err := server.Store.ListCommanders(r.Context())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"region": region.Current(), "data": filepath.Base(gamedata.Dir()),
		"bosses": bossRows, "surveys": surveyRows, "commanders": len(commanders), "offset": game.Clock(now).Offset,
	}, nil
}

// handleResetMark forgets when a boss's HP reached a mark.
func (server *Server) handleResetMark(r *http.Request) (any, error) {
	var body struct {
		ActivityID uint32 `json:"activity_id"`
		BossID     uint32 `json:"boss_id"`
		Mark       uint32 `json:"mark"`
	}
	if err := readJSON(r, &body); err != nil {
		return nil, err
	}
	deleted, err := server.Game.ResetBossMark(r.Context(), body.ActivityID, body.BossID, body.Mark)
	if err != nil {
		return nil, err
	}
	if !deleted {
		return nil, notFound("no such mark")
	}
	return struct{}{}, nil
}

// handleResetSurvey lets every commander take a survey again.
func (server *Server) handleResetSurvey(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	reset, err := server.Game.ResetSurvey(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if reset == 0 {
		return nil, notFound("no one took survey " + itoa(id))
	}
	return map[string]int64{"reset": reset}, nil
}
