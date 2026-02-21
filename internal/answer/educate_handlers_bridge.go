package answer

import (
	answereducate "github.com/ggmolly/belfast/internal/answer/educate"
	"github.com/ggmolly/belfast/internal/connection"
)

const (
	educateFlagHomeEventBase    = uint32(270140000)
	educateFlagSpecialEventBase = uint32(270270000)
	educateFlagDiscountBase     = uint32(270271000)
	educateFlagTargetAwardBase  = uint32(270350000)

	childSiteCategory             = "ShareCfg/child_site.json"
	childSiteOptionCategory       = "ShareCfg/child_site_option.json"
	childSiteOptionBranchCategory = "ShareCfg/child_site_option_branch.json"
	childTaskCategory             = "ShareCfg/child_task.json"
	childTargetSetCategory        = "ShareCfg/child_target_set.json"
	childDataCategory             = "ShareCfg/child_data.json"
	childEndingCategory           = "ShareCfg/child_ending.json"
	secretarySpecialShipCategory  = "ShareCfg/secretary_special_ship.json"
)

func educateFlagID(base uint32, id uint32) uint32 {
	return answereducate.EducateFlagID(base, id)
}

func hasEducateFlag(commanderID uint32, flagID uint32) (bool, error) {
	return answereducate.HasEducateFlag(commanderID, flagID)
}

func EducateExecutePlans(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateExecutePlans(buffer, client)
}

func EducateGetEvents(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateGetEvents(buffer, client)
}

func EducateGetPlans(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateGetPlans(buffer, client)
}

func EducateGetTargetAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateGetTargetAward(buffer, client)
}

func EducateUpgradeFavor(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateUpgradeFavor(buffer, client)
}

func EducateTriggerEnd(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateTriggerEnd(buffer, client)
}

func EducateGetEndings(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateGetEndings(buffer, client)
}

func EducateSetTarget(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateSetTarget(buffer, client)
}

func EducateSubmitTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateSubmitTask(buffer, client)
}

func EducateSetCall(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateSetCall(buffer, client)
}

func EducateAddTaskProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateAddTaskProgress(buffer, client)
}

func EducateAddExtraAttr(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateAddExtraAttr(buffer, client)
}

func ChangeEducateCharacter(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.ChangeEducateCharacter(buffer, client)
}

func EducateMapSiteOperate(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateMapSiteOperate(buffer, client)
}

func EducateRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateRefresh(buffer, client)
}

func EducateRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateRequest(buffer, client)
}

func EducateRequestOption(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateRequestOption(buffer, client)
}

func EducateRequestShopData(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateRequestShopData(buffer, client)
}

func EducateReset(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateReset(buffer, client)
}

func EducateShopping(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateShopping(buffer, client)
}

func EducateTriggerEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateTriggerEvent(buffer, client)
}

func EducateTriggerSpecEvent(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answereducate.EducateTriggerSpecEvent(buffer, client)
}
