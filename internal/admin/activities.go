package admin

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ggmolly/belfast/internal/game"
	"github.com/ggmolly/belfast/internal/gamedata"
)

// registerActivities adds the back-office's Activities routes.
func (server *Server) registerActivities(api apiFunc) {
	api("GET /api/commanders/{id}/activities/detail", server.handleActivityDetail)
	api("POST /api/commanders/{id}/activities/task", server.handleActivityTask)
	api("POST /api/commanders/{id}/activities/task/reset", server.handleActivityTaskReset)
	api("POST /api/commanders/{id}/activities/cruise", server.handleActivityCruise)
	api("POST /api/commanders/{id}/activities/cruise/award", server.handleActivityCruiseAward)
	api("POST /api/commanders/{id}/activities/signs", server.handleActivitySigns)
	api("POST /api/commanders/{id}/activities/score", server.handleActivityScore)
}

type activityTask struct {
	ID       uint32   `json:"id"`
	Desc     string   `json:"desc"`
	Progress uint32   `json:"progress"`
	Target   uint32   `json:"target"`
	Claimed  bool     `json:"claimed"`
	Held     bool     `json:"held"`
	Awards   []reward `json:"awards"`
}

type activityGroup struct {
	ID     uint32         `json:"id"`
	Label  string         `json:"label"`
	Locked bool           `json:"locked"`
	Tasks  []activityTask `json:"tasks"`
}

type activityLevel struct {
	Level       uint32 `json:"level"`
	Pt          uint32 `json:"pt"`
	Key         bool   `json:"key"`
	Free        reward `json:"free"`
	Paid        reward `json:"paid"`
	FreeClaimed bool   `json:"free_claimed"`
	PaidClaimed bool   `json:"paid_claimed"`
}

type activityCruise struct {
	Item   reward          `json:"item"`
	Pt     uint32          `json:"pt"`
	Shown  uint32          `json:"shown"`
	Max    uint32          `json:"max"`
	Level  uint32          `json:"level"`
	Paid   bool            `json:"paid"`
	Levels []activityLevel `json:"levels"`
}

type activitySigns struct {
	Count   uint32     `json:"count"`
	Max     uint32     `json:"max"`
	Days    [][]reward `json:"days"`
	Last    int64      `json:"last,omitempty"`
	Year    uint32     `json:"year,omitempty"`
	Month   uint32     `json:"month,omitempty"`
	Dates   []uint32   `json:"dates"`
	Resigns uint32     `json:"resigns"`
}

type activityBracket struct {
	ID    uint32 `json:"id"`
	Lower uint32 `json:"lower"`
	Upper uint32 `json:"upper"`
}

type activityScore struct {
	Score    uint32            `json:"score"`
	Best     uint32            `json:"best"`
	Rank     uint32            `json:"rank"`
	Brackets []activityBracket `json:"brackets"`
}

type juusPost struct {
	ID        uint32 `json:"id"`
	Title     string `json:"title,omitempty"`
	Ship      string `json:"ship"`
	Icon      string `json:"icon,omitempty"`
	Rarity    uint32 `json:"rarity"`
	Liked     bool   `json:"liked"`
	Read      bool   `json:"read"`
	Commented bool   `json:"commented"`
}

type juusTopic struct {
	ID     uint32 `json:"id"`
	Name   string `json:"name"`
	Ship   string `json:"ship"`
	Icon   string `json:"icon,omitempty"`
	Rarity uint32 `json:"rarity"`
	Read   bool   `json:"read"`
	Time   uint32 `json:"time"`
}

type activityJuus struct {
	Posts  []juusPost  `json:"posts"`
	Topics []juusTopic `json:"topics"`
}

// activityDetail is one activity as its client page shows it: Kind says
// which of the parts it has ("" when the screen shows the stored rows).
type activityDetail struct {
	Kind   string            `json:"kind"`
	Labels map[string]string `json:"labels"`
	Groups []activityGroup   `json:"groups"`
	Cruise *activityCruise   `json:"cruise,omitempty"`
	Signs  *activitySigns    `json:"signs,omitempty"`
	Score  *activityScore    `json:"score,omitempty"`
	Juus   *activityJuus     `json:"juus,omitempty"`
}

// activityTips are the gametip keys of what the pages call things:
// WorldCruiseTaskPage's week tabs and task states, the pass's tracks.
var activityTips = map[string]string{
	"week":     "cruise_task_week",   // Week $1
	"seasonal": "cruise_tip_skin",    // Seasonal Missions, the group_mask 0 tab
	"get":      "task_get",           // a task done, its reward to collect
	"got":      "task_got",           // a task claimed
	"phase":    "cruise_phase_title", // the pass's level
	"base":     "cruise_tip_base",    // - Base Rewards -
	"bonus":    "cruise_tip_upgrade", // - Bonus Rewards -
}

// activityNames are the config names the Activities detail reads, once.
type activityNames struct {
	labels  map[string]string
	posts   map[uint32]juusPostRow
	topics  map[uint32]juusTopicRow
	loadErr error
}

type juusPostRow struct {
	ShipGroup uint32 `json:"ship_group"`
	Title     string `json:"title"`
}

type juusTopicRow struct {
	Name      string `json:"name"`
	ShipGroup uint32 `json:"ship_group"`
}

var loadActivityNames = sync.OnceValue(func() *activityNames {
	names := &activityNames{labels: map[string]string{}}
	tips, err := gamedata.Gametip()
	if err != nil {
		names.loadErr = err
		return names
	}
	for label, key := range activityTips {
		names.labels[label] = strings.TrimSpace(strings.Trim(tips[key], "-"))
	}
	if names.posts, err = gamedata.Table[juusPostRow]("activity_ins_template"); err != nil {
		names.loadErr = err
		return names
	}
	if names.topics, err = gamedata.Table[juusTopicRow]("activity_ins_chat_group"); err != nil {
		names.loadErr = err
		return names
	}
	return names
})

// activityID reads the activity key ("activity:<id>") of a body or query.
func activityID(key string) (uint32, bool) {
	kind, arg, _ := strings.Cut(key, ":")
	id, err := strconv.ParseUint(arg, 10, 32)
	return uint32(id), kind == "activity" && err == nil && id != 0
}

func (server *Server) activityDrop(drop [3]uint32) reward {
	return server.namedDrops([][3]uint32{drop})[0]
}

// handleActivityDetail is one activity or system of the commander, read the
// way its page reads it: ?key= as the list names it.
func (server *Server) handleActivityDetail(r *http.Request) (any, error) {
	commander, err := server.commander(r)
	if err != nil {
		return nil, err
	}
	names := loadActivityNames()
	if names.loadErr != nil {
		return nil, names.loadErr
	}
	key := r.URL.Query().Get("key")
	detail := activityDetail{Labels: names.labels, Groups: []activityGroup{}}
	if key == "juus" {
		juus, err := server.juusDetail(r, commander.ID, names)
		if err != nil {
			return nil, err
		}
		detail.Kind, detail.Juus = "juus", juus
		return detail, nil
	}
	id, ok := activityID(key)
	if !ok {
		return detail, nil
	}
	view, ok, err := server.Game.AdminActivity(r.Context(), commander.ID, id)
	if err != nil || !ok {
		return detail, err
	}
	detail.Kind = view.Kind
	for _, group := range view.Groups {
		row := activityGroup{ID: group.ID, Locked: group.Locked, Tasks: []activityTask{}}
		switch {
		case view.Kind == game.AdminActivityCruise && group.ID == 0:
			row.Label = names.labels["seasonal"]
		case view.Kind == game.AdminActivityCruise:
			row.Label = strings.ReplaceAll(names.labels["week"], "$1", itoa(group.ID))
		case view.Kind == game.AdminActivityTaskList:
			// MonthSignPage and the task list pages write "Day " .. n
			row.Label = "Day " + itoa(group.ID)
		}
		for _, task := range group.Tasks {
			row.Tasks = append(row.Tasks, activityTask{ID: task.ID, Desc: task.Desc, Progress: task.Progress, Target: task.Target, Claimed: task.Claimed, Held: task.Held, Awards: server.namedDrops(task.Awards)})
		}
		detail.Groups = append(detail.Groups, row)
	}
	if cruise := view.Cruise; cruise != nil {
		row := &activityCruise{Item: server.activityDrop([3]uint32{8, cruise.Item, cruise.Pt}), Pt: cruise.Pt, Shown: cruise.Shown, Paid: cruise.Paid, Levels: []activityLevel{}}
		for _, level := range cruise.Levels {
			row.Max = max(row.Max, level.Pt)
			if level.Pt <= cruise.Shown {
				row.Level = level.Level
			}
			row.Levels = append(row.Levels, activityLevel{Level: level.Level, Pt: level.Pt, Key: level.Key, Free: server.activityDrop(level.Free), Paid: server.activityDrop(level.Paid), FreeClaimed: level.FreeClaimed, PaidClaimed: level.PaidClaimed})
		}
		detail.Cruise = row
	}
	if signs := view.Signs; signs != nil {
		row := &activitySigns{Count: signs.Count, Max: uint32(len(signs.Days)), Last: signs.Last, Year: signs.Year, Month: signs.Month, Dates: signs.Dates, Resigns: signs.Resigns, Days: [][]reward{}}
		if row.Dates == nil {
			row.Dates = []uint32{}
		}
		for _, day := range signs.Days {
			row.Days = append(row.Days, server.namedDrops(day))
		}
		detail.Signs = row
	}
	if score := view.Score; score != nil {
		row := &activityScore{Score: score.Score, Best: score.Best, Brackets: []activityBracket{}}
		for _, bracket := range score.Brackets {
			if score.Score >= bracket.Lower {
				row.Rank = bracket.ID
			}
			row.Brackets = append(row.Brackets, activityBracket{ID: bracket.ID, Lower: bracket.Lower, Upper: bracket.Upper})
		}
		detail.Score = row
	}
	return detail, nil
}

// juusDetail is the commander's Juustagram: the posts pushed to them
// (activity_ins_template, its ship and message) and the chat topics
// unlocked (activity_ins_chat_group).
func (server *Server) juusDetail(r *http.Request, commanderID uint32, names *activityNames) (*activityJuus, error) {
	posts, err := server.Store.LifeListCommanderJuusPosts(r.Context(), commanderID)
	if err != nil {
		return nil, err
	}
	topics, err := server.Store.LifeListCommanderJuusTopics(r.Context(), commanderID)
	if err != nil {
		return nil, err
	}
	juus := &activityJuus{Posts: []juusPost{}, Topics: []juusTopic{}}
	for _, post := range posts {
		config := names.posts[post.PostID]
		ship := server.groupShip(config.ShipGroup)
		juus.Posts = append(juus.Posts, juusPost{ID: post.PostID, Title: config.Title, Ship: ship.Name, Icon: ship.Icon, Rarity: ship.Rarity, Liked: post.IsGood, Read: post.IsRead, Commented: post.CommentDiscuss != 0})
	}
	sort.SliceStable(juus.Posts, func(i, j int) bool { return juus.Posts[i].ID > juus.Posts[j].ID })
	for _, topic := range topics {
		config := names.topics[topic.TopicID]
		ship := server.groupShip(config.ShipGroup)
		juus.Topics = append(juus.Topics, juusTopic{ID: topic.TopicID, Name: config.Name, Ship: ship.Name, Icon: ship.Icon, Rarity: ship.Rarity, Read: topic.ReadFlag, Time: topic.OpTime})
	}
	sort.SliceStable(juus.Topics, func(i, j int) bool { return juus.Topics[i].Time > juus.Topics[j].Time })
	return juus, nil
}

// groupShip is a ship group's first template, nothing for group 0.
func (server *Server) groupShip(group uint32) named {
	if group == 0 {
		return named{}
	}
	return server.names.shown(dropShip, group*10+1)
}

// activityEdit runs edit on the {id} commander's activity its body's key
// names (see commanderEdit).
func activityEdit[T any](server *Server, r *http.Request, key func(T) string, edit func(commander, activity uint32, body T) (bool, error)) (any, error) {
	return commanderEdit(server, r, func(_ context.Context, id uint32, body T) (bool, error) {
		activity, ok := activityID(key(body))
		if !ok {
			return false, badRequest("not an activity")
		}
		return edit(id, activity, body)
	})
}

type taskBody struct {
	Key      string `json:"key"`
	Task     uint32 `json:"task"`
	Progress uint32 `json:"progress"`
	Claimed  bool   `json:"claimed"`
}

// handleActivityTask sets a task's progress and claimed state: {key, task,
// progress, claimed}.
func (server *Server) handleActivityTask(r *http.Request) (any, error) {
	return activityEdit(server, r, func(b taskBody) string { return b.Key }, func(commander, activity uint32, b taskBody) (bool, error) {
		return server.Game.AdminSetActivityTask(r.Context(), commander, activity, b.Task, b.Progress, b.Claimed)
	})
}

// handleActivityTaskReset starts a task over: {key, task}.
func (server *Server) handleActivityTaskReset(r *http.Request) (any, error) {
	return activityEdit(server, r, func(b taskBody) string { return b.Key }, func(commander, activity uint32, b taskBody) (bool, error) {
		return server.Game.AdminResetActivityTask(r.Context(), commander, activity, b.Task)
	})
}

type cruiseBody struct {
	Key     string  `json:"key"`
	Pt      *uint32 `json:"pt"`
	Paid    *bool   `json:"paid"`
	Level   uint32  `json:"level"`
	Claimed bool    `json:"claimed"`
}

// handleActivityCruise sets the cruise pass's pt and bonus track: {key,
// pt?, paid?}.
func (server *Server) handleActivityCruise(r *http.Request) (any, error) {
	return activityEdit(server, r, func(b cruiseBody) string { return b.Key }, func(commander, activity uint32, b cruiseBody) (bool, error) {
		return server.Game.AdminSetCruise(r.Context(), commander, activity, b.Pt, b.Paid)
	})
}

// handleActivityCruiseAward marks a base level claimed or not, 0 every
// level reached: {key, level, claimed}.
func (server *Server) handleActivityCruiseAward(r *http.Request) (any, error) {
	return activityEdit(server, r, func(b cruiseBody) string { return b.Key }, func(commander, activity uint32, b cruiseBody) (bool, error) {
		return server.Game.AdminSetCruiseAward(r.Context(), commander, activity, b.Level, b.Claimed)
	})
}

type signsBody struct {
	Key   string `json:"key"`
	Count uint32 `json:"count"`
}

// handleActivitySigns sets how many days of a login calendar are signed:
// {key, count}.
func (server *Server) handleActivitySigns(r *http.Request) (any, error) {
	return activityEdit(server, r, func(b signsBody) string { return b.Key }, func(commander, activity uint32, b signsBody) (bool, error) {
		return server.Game.AdminSetSigns(r.Context(), commander, activity, b.Count)
	})
}

type scoreBody struct {
	Key   string `json:"key"`
	Score uint32 `json:"score"`
	Best  uint32 `json:"best"`
}

// handleActivityScore sets a Cheater's Tavern season's score and best:
// {key, score, best}.
func (server *Server) handleActivityScore(r *http.Request) (any, error) {
	return activityEdit(server, r, func(b scoreBody) string { return b.Key }, func(commander, activity uint32, b scoreBody) (bool, error) {
		return server.Game.AdminSetScore(r.Context(), commander, activity, b.Score, b.Best)
	})
}
