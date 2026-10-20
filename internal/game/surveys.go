package game

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// Questionnaires (controller/command/survey, ActivityProxy:isSurveyOpen,
// view/activity/subpages/surveypage.lua). A survey runs as an
// ActivityConst.ACTIVITY_TYPE_SURVEY activity: config_id is its
// survey_data_template row, config_data {1, level} the commander level it
// needs. The page links to the survey site (SdkMgr:Survey) and shows the
// row's bonus; the activity goes away once the server says the survey is
// done (SurveyStateCommand, or SC_11028 pushed). The official server learns
// it from the survey site; belfast can't, so opening the survey counts as
// taking it. Nothing was captured: the replies are built from the Lua alone.

const activityTypeSurvey = 101

// survey_data_template row
type surveyConfig struct {
	Name  string      `json:"name"`
	Bonus [][3]uint32 `json:"bonus"` // DROPINFO type, id, count
}

var surveys map[uint32]surveyConfig

// openSurvey tells whether survey is the one of a survey activity running at
// now that the commander's level lets them take (ActivityProxy:isSurveyOpen).
func openSurvey(survey uint32, level uint32, now time.Time) bool {
	if _, ok := surveys[survey]; !ok {
		return false
	}
	for _, config := range openActivities(now) {
		if config.Type != activityTypeSurvey || config.ConfigID != survey {
			continue
		}
		var data []uint32
		if json.Unmarshal(config.ConfigData, &data) == nil && len(data) == 2 && data[0] == 1 && level >= data[1] {
			return true
		}
	}
	return false
}

// surveyDone is SC_11028.result: 1 once the commander took survey, else 0.
func surveyDone(ctx context.Context, queries *db.Queries, commanderID, survey uint32) (uint32, error) {
	done, err := queries.CountSurveysDone(ctx, db.CountSurveysDoneParams{CommanderID: commanderID, SurveyID: survey})
	if done > 0 {
		return 1, err
	}
	return 0, err
}

// handleSurvey answers SurveyCommand (CS_11025), sent from the survey page's
// button: on result 0 the client opens the survey site. The first time, the
// survey counts as done: its bonus is mailed and SC_11028 {1} follows the
// reply so the page goes away. A survey not running or above the
// commander's level is refused (errorTip("", result)).
func (game *Game) handleSurvey(client *connection.Client, req *protobuf.CS_11025) error {
	ctx, now := context.Background(), time.Now()
	survey := req.GetSurveyId()
	taken := false
	err := game.tx(ctx, client, func(tx *Tx) error {
		commander, err := tx.GetCommander(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		if !openSurvey(survey, commander.Level, now) {
			return errRefused
		}
		done, err := surveyDone(ctx, tx.Queries, client.CommanderID, survey)
		if err != nil || done != 0 {
			return err
		}
		if err := tx.AddSurveyDone(ctx, db.AddSurveyDoneParams{CommanderID: client.CommanderID, SurveyID: survey, DoneAt: now.Unix()}); err != nil {
			return err
		}
		taken = true
		config := surveys[survey]
		mail := Mail{Title: config.Name}
		for _, bonus := range config.Bonus {
			mail.Attachments = append(mail.Attachments, dropInfo(bonus[0], bonus[1], bonus[2]))
		}
		if mail.Title == "" || len(mail.Attachments) == 0 {
			return nil
		}
		return tx.SendMail(mail)
	})
	if err != nil && !refused(err) {
		return err
	}
	if taken {
		if err := client.PushAfterReply(&protobuf.SC_11028{Result: proto.Uint32(1)}); err != nil {
			return err
		}
	}
	return client.Send(&protobuf.SC_11026{Result: result(err)})
}

// handleSurveyState answers SurveyStateCommand (CS_11027), sent after the
// login when a survey is open: result 1 when the commander took it, 0 when
// not (ActivityProxy:setSurveyState).
func (game *Game) handleSurveyState(client *connection.Client, req *protobuf.CS_11027) error {
	done, err := surveyDone(context.Background(), game.DB.Queries, client.CommanderID, req.GetSurveyId())
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11028{Result: proto.Uint32(done)})
}
