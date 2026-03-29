package educate

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/config"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	legacyEducateResultOK      = uint32(0)
	legacyEducateResultFailure = uint32(1)

	childSiteCategory             = "ShareCfg/child_site.json"
	childSiteOptionCategory       = "ShareCfg/child_site_option.json"
	childSiteOptionBranchCategory = "ShareCfg/child_site_option_branch.json"
	childTaskCategory             = "ShareCfg/child_task.json"
	childTargetSetCategory        = "ShareCfg/child_target_set.json"
	childDataCategory             = "ShareCfg/child_data.json"
	childEndingCategory           = "ShareCfg/child_ending.json"
	secretarySpecialShipCategory  = "ShareCfg/secretary_special_ship.json"

	legacyEducateCallNameMin = 4
	legacyEducateCallNameMax = 14
)

type legacyChildSite struct {
	ID           uint32            `json:"id"`
	Option       []uint32          `json:"option"`
	OptionRandom []json.RawMessage `json:"option_random"`
}

type legacyChildSiteOption struct {
	ID         uint32     `json:"id"`
	Type       uint32     `json:"type"`
	Result     []uint32   `json:"result"`
	Cost       [][]uint32 `json:"cost"`
	CountLimit []uint32   `json:"count_limit"`
}

type legacyChildTask struct {
	ID          uint32   `json:"id"`
	Type1       uint32   `json:"type_1"`
	Arg         uint32   `json:"arg"`
	DropDisplay []uint32 `json:"drop_display"`
}

type legacyChildTargetSet struct {
	ID uint32 `json:"id"`
}

type legacyChildData struct {
	ID        uint32   `json:"id"`
	Attr2List []uint32 `json:"attr_2_list"`
	Attr2Add  uint32   `json:"attr_2_add"`
	FavorLv   uint32   `json:"favor_level"`
}

type legacySecretarySpecialShip struct {
	ID uint32 `json:"id"`
}

func EducateUpgradeFavor(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27006
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27007, err
	}

	response := protobuf.SC_27007{
		Result: proto.Uint32(legacyEducateResultFailure),
		Drops:  []*protobuf.CHILD_DROP{},
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		response.Result = proto.Uint32(legacyEducateResultFailure)
		return client.SendMessage(27007, &response)
	}

	maxFavor := uint32(0)
	if childData, ok, err := loadLegacyChildData(); err == nil && ok {
		maxFavor = childData.FavorLv
	}
	if maxFavor > 0 && state.FavorLv >= maxFavor {
		return client.SendMessage(27007, &response)
	}

	state.FavorLv++
	if err := orm.SaveLegacyEducateState(state); err != nil {
		return client.SendMessage(27007, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	return client.SendMessage(27007, &response)
}

func EducateTriggerEnd(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27008
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27009, err
	}

	response := protobuf.SC_27009{Result: proto.Uint32(legacyEducateResultFailure)}
	endingID := payload.GetEndingId()
	if endingID == 0 {
		return client.SendMessage(27009, &response)
	}
	if ok, err := legacyConfigExists(childEndingCategory, endingID); err != nil || !ok {
		return client.SendMessage(27009, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27009, &response)
	}
	state.Endings = appendUniqueUint32(state.Endings, endingID)
	for _, qualifiedID := range payload.GetQualifiedId() {
		state.Qualifieds = appendUniqueUint32(state.Qualifieds, qualifiedID)
	}
	if err := orm.SaveLegacyEducateState(state); err != nil {
		return client.SendMessage(27009, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	return client.SendMessage(27009, &response)
}

func EducateGetEndings(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27010
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27011, err
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return 0, 27011, err
	}

	response := protobuf.SC_27011{
		Endings:    append([]uint32{}, state.Endings...),
		Qualifieds: append([]uint32{}, state.Qualifieds...),
	}
	return client.SendMessage(27011, &response)
}

func EducateSetTarget(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27019
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27020, err
	}

	response := protobuf.SC_27020{Result: proto.Uint32(legacyEducateResultFailure)}
	targetID := payload.GetId()
	if targetID == 0 {
		return client.SendMessage(27020, &response)
	}
	if _, ok, err := loadLegacyConfigByID[legacyChildTargetSet](childTargetSetCategory, targetID); err != nil || !ok {
		return client.SendMessage(27020, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27020, &response)
	}
	state.TargetID = targetID
	if err := orm.SaveLegacyEducateState(state); err != nil {
		return client.SendMessage(27020, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	return client.SendMessage(27020, &response)
}

func EducateSubmitTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27023
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27024, err
	}

	response := protobuf.SC_27024{
		Result: proto.Uint32(legacyEducateResultFailure),
		Awards: []*protobuf.CHILD_DROP{},
	}

	taskID := payload.GetId()
	if taskID == 0 || payload.GetSystem() == 0 {
		return client.SendMessage(27024, &response)
	}
	taskConfig, ok, err := loadLegacyConfigByID[legacyChildTask](childTaskCategory, taskID)
	if err != nil || !ok || taskConfig.Type1 != payload.GetSystem() {
		return client.SendMessage(27024, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27024, &response)
	}
	if state.TaskProgress[taskID] < taskConfig.Arg {
		return client.SendMessage(27024, &response)
	}

	delete(state.TaskProgress, taskID)
	if err := orm.SaveLegacyEducateState(state); err != nil {
		return client.SendMessage(27024, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	if len(taskConfig.DropDisplay) >= 3 {
		response.Awards = append(response.Awards, buildLegacyChildDrop(taskConfig.DropDisplay[0], taskConfig.DropDisplay[1], int32(taskConfig.DropDisplay[2])))
	}
	return client.SendMessage(27024, &response)
}

func EducateSetCall(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27031
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27032, err
	}

	response := protobuf.SC_27032{Result: proto.Uint32(legacyEducateResultFailure)}
	name := strings.TrimSpace(payload.GetName())
	if !isValidLegacyCallName(name) {
		return client.SendMessage(27032, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27032, &response)
	}
	state.CallName = name
	if err := orm.SaveLegacyEducateState(state); err != nil {
		return client.SendMessage(27032, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	return client.SendMessage(27032, &response)
}

func EducateAddTaskProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27037
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27038, err
	}

	response := protobuf.SC_27038{Result: proto.Uint32(legacyEducateResultFailure)}
	if payload.GetType_1() < 1 || payload.GetType_1() > 3 || len(payload.GetProgresses()) == 0 {
		return client.SendMessage(27038, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27038, &response)
	}

	updatedTasks := make([]*protobuf.CHILD_TASK, 0, len(payload.GetProgresses()))
	for _, progress := range payload.GetProgresses() {
		if progress.GetTaskId() == 0 || progress.GetProgress() == 0 {
			return client.SendMessage(27038, &response)
		}
		taskConfig, ok, err := loadLegacyConfigByID[legacyChildTask](childTaskCategory, progress.GetTaskId())
		if err != nil || !ok || taskConfig.Type1 != payload.GetType_1() {
			return client.SendMessage(27038, &response)
		}
		newProgress := state.TaskProgress[progress.GetTaskId()] + progress.GetProgress()
		if taskConfig.Arg > 0 && newProgress > taskConfig.Arg {
			newProgress = taskConfig.Arg
		}
		state.TaskProgress[progress.GetTaskId()] = newProgress
		updatedTasks = append(updatedTasks, &protobuf.CHILD_TASK{Id: proto.Uint32(progress.GetTaskId()), Progress: proto.Uint32(newProgress)})
	}

	if err := orm.SaveLegacyEducateState(state); err != nil {
		return client.SendMessage(27038, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	bytesWritten, packetID, err := client.SendMessage(27038, &response)
	if err != nil {
		return bytesWritten, packetID, err
	}
	if len(updatedTasks) > 0 {
		if _, _, err := client.SendMessage(27025, &protobuf.SC_27025{Tasks: updatedTasks}); err != nil {
			return bytesWritten, packetID, err
		}
	}
	return bytesWritten, packetID, nil
}

func EducateAddExtraAttr(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27039
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27040, err
	}

	response := protobuf.SC_27040{Result: proto.Uint32(legacyEducateResultFailure)}
	childData, ok, err := loadLegacyChildData()
	if err != nil || !ok {
		return client.SendMessage(27040, &response)
	}
	if !containsUint32(childData.Attr2List, payload.GetAttrId()) {
		return client.SendMessage(27040, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27040, &response)
	}
	if state.HadAdjustment {
		return client.SendMessage(27040, &response)
	}

	state.Attrs[payload.GetAttrId()] += childData.Attr2Add
	state.HadAdjustment = true
	if err := orm.SaveLegacyEducateState(state); err != nil {
		return client.SendMessage(27040, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	return client.SendMessage(27040, &response)
}

func ChangeEducateCharacter(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27041
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27042, err
	}

	response := protobuf.SC_27042{Result: proto.Uint32(legacyEducateResultFailure)}
	endingID := payload.GetEndingId()
	if endingID == 0 {
		return client.SendMessage(27042, &response)
	}
	if _, ok, err := loadLegacyConfigByID[legacySecretarySpecialShip](secretarySpecialShipCategory, endingID); err != nil || !ok {
		return client.SendMessage(27042, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27042, &response)
	}
	if !containsUint32(state.Endings, endingID) {
		return client.SendMessage(27042, &response)
	}

	if err := orm.UpdateCommanderChildDisplay(client.Commander.CommanderID, endingID); err != nil {
		return client.SendMessage(27042, &response)
	}
	client.Commander.ChildDisplay = endingID

	response.Result = proto.Uint32(legacyEducateResultOK)
	return client.SendMessage(27042, &response)
}

func EducateMapSiteOperate(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27004
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27005, err
	}

	response := protobuf.SC_27005{
		Result:     proto.Uint32(legacyEducateResultFailure),
		Drops:      []*protobuf.CHILD_DROP{},
		EventDrops: []*protobuf.CHILD_DROP{},
		Events:     []uint32{},
		BranchId:   proto.Uint32(0),
	}

	site, ok, err := loadLegacyConfigByID[legacyChildSite](childSiteCategory, payload.GetSiteid())
	if err != nil || !ok {
		return client.SendMessage(27005, &response)
	}
	if !legacySiteHasOption(site, payload.GetOptionid()) {
		return client.SendMessage(27005, &response)
	}

	option, ok, err := loadLegacyConfigByID[legacyChildSiteOption](childSiteOptionCategory, payload.GetOptionid())
	if err != nil || !ok || option.Type != 2 {
		return client.SendMessage(27005, &response)
	}

	state, err := orm.GetOrCreateLegacyEducateState(client.Commander.CommanderID)
	if err != nil {
		return client.SendMessage(27005, &response)
	}

	if len(option.CountLimit) >= 1 {
		if state.OptionRecords[payload.GetOptionid()] >= option.CountLimit[0] {
			return client.SendMessage(27005, &response)
		}
	}

	for _, cost := range option.Cost {
		if len(cost) < 3 {
			continue
		}
		if cost[0] != 2 {
			continue
		}
		if state.Resources[cost[1]] < int32(cost[2]) {
			return client.SendMessage(27005, &response)
		}
	}

	branchID := uint32(0)
	for _, candidate := range option.Result {
		if ok, err := legacyConfigExists(childSiteOptionBranchCategory, candidate); err == nil && ok {
			branchID = candidate
			break
		}
	}
	if branchID == 0 {
		return client.SendMessage(27005, &response)
	}

	for _, cost := range option.Cost {
		if len(cost) < 3 || cost[0] != 2 {
			continue
		}
		state.Resources[cost[1]] -= int32(cost[2])
	}
	state.OptionRecords[payload.GetOptionid()]++
	if err := orm.SaveLegacyEducateState(state); err != nil {
		return client.SendMessage(27005, &response)
	}

	response.Result = proto.Uint32(legacyEducateResultOK)
	response.BranchId = proto.Uint32(branchID)
	return client.SendMessage(27005, &response)
}

func isValidLegacyCallName(name string) bool {
	if name == "" {
		return false
	}
	nameLength := utf8.RuneCountInString(name)
	if nameLength < legacyEducateCallNameMin || nameLength > legacyEducateCallNameMax {
		return false
	}
	createConfig := config.Current().CreatePlayer
	if len(createConfig.NameBlacklist) > 0 {
		lowerName := strings.ToLower(name)
		for _, blocked := range createConfig.NameBlacklist {
			blocked = strings.TrimSpace(blocked)
			if blocked == "" {
				continue
			}
			if strings.Contains(lowerName, strings.ToLower(blocked)) {
				return false
			}
		}
	}
	if createConfig.NameIllegalPattern != "" {
		matcher, err := regexp.Compile(createConfig.NameIllegalPattern)
		if err != nil {
			return false
		}
		if matcher.MatchString(name) {
			return false
		}
	}
	return true
}

func buildLegacyChildDrop(dropType uint32, id uint32, number int32) *protobuf.CHILD_DROP {
	return &protobuf.CHILD_DROP{
		Type:   proto.Uint32(dropType),
		Id:     proto.Uint32(id),
		Number: proto.Int32(number),
	}
}

func legacySiteHasOption(site *legacyChildSite, optionID uint32) bool {
	if site == nil {
		return false
	}
	if containsUint32(site.Option, optionID) {
		return true
	}
	for _, row := range site.OptionRandom {
		var groups []json.RawMessage
		if err := json.Unmarshal(row, &groups); err != nil {
			continue
		}
		for _, group := range groups {
			var values []json.RawMessage
			if err := json.Unmarshal(group, &values); err != nil || len(values) == 0 {
				continue
			}
			var candidate uint32
			if err := json.Unmarshal(values[0], &candidate); err != nil {
				continue
			}
			if candidate == optionID {
				return true
			}
		}
	}
	return false
}

func loadLegacyChildData() (*legacyChildData, bool, error) {
	entry, err := orm.GetConfigEntry(childDataCategory, "1")
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var configData legacyChildData
	if err := json.Unmarshal(entry.Data, &configData); err != nil {
		return nil, false, err
	}
	return &configData, true, nil
}

func legacyConfigExists(category string, id uint32) (bool, error) {
	_, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func loadLegacyConfigByID[T any](category string, id uint32) (*T, bool, error) {
	entry, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if db.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var configData T
	if err := json.Unmarshal(entry.Data, &configData); err != nil {
		return nil, false, err
	}
	return &configData, true, nil
}
