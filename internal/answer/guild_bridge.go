package answer

import (
	"github.com/ggmolly/belfast/internal/answer/guild"
	"github.com/ggmolly/belfast/internal/connection"
)

func AcceptGuildJoinRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.AcceptGuildJoinRequest(buffer, client)
}

func GuildListRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildListRefresh(buffer, client)
}

func GuildCommitDonate(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildCommitDonate(buffer, client)
}

func GuildBuySupply(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildBuySupply(buffer, client)
}

func GuildGetSupplyAwardCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildGetSupplyAwardCommandResponse(buffer, client)
}

func GuildFetchCapitalLogCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildFetchCapitalLogCommandResponse(buffer, client)
}

func GuildSelectWeeklyTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildSelectWeeklyTask(buffer, client)
}

func GuildUpgradeTechnologyCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildUpgradeTechnologyCommandResponse(buffer, client)
}

func GuildStartTechGroupCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildStartTechGroupCommandResponse(buffer, client)
}

func GuildFetchWeeklyTaskProgressCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildFetchWeeklyTaskProgressCommandResponse(buffer, client)
}

func GuildFetchCapitalCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildFetchCapitalCommandResponse(buffer, client)
}

func GuildGetRankCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildGetRankCommandResponse(buffer, client)
}

func GuildGetUserInfoCommand(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildGetUserInfoCommand(buffer, client)
}

func GuildFetchBossCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildFetchBossCommandResponse(buffer, client)
}

func GuildGetBossRankCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildGetBossRankCommandResponse(buffer, client)
}

func GuildGetBossInfoCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildGetBossInfoCommandResponse(buffer, client)
}

func GuildUpdateNodeAnimFlagCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildUpdateNodeAnimFlagCommandResponse(buffer, client)
}

func GuildUpdateBossMissionFleetCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildUpdateBossMissionFleetCommandResponse(buffer, client)
}

func GuildUpdateAssaultFleetCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildUpdateAssaultFleetCommandResponse(buffer, client)
}

func GuildGetAssaultFleetCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildGetAssaultFleetCommandResponse(buffer, client)
}

func GetMyAssaultFleetCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GetMyAssaultFleetCommandResponse(buffer, client)
}

func MarkAssaultShipRecommendCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.MarkAssaultShipRecommendCommandResponse(buffer, client)
}

func GuildRefreshAssaultRecommendationsCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildRefreshAssaultRecommendationsCommandResponse(buffer, client)
}

func GuildJoinEventCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildJoinEventCommandResponse(buffer, client)
}

func GuildGetReportsCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildGetReportsCommandResponse(buffer, client)
}

func GuildGetReportRankCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildGetReportRankCommandResponse(buffer, client)
}

func GuildGetActivationEventCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildGetActivationEventCommandResponse(buffer, client)
}

func GuildApply(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildApply(buffer, client)
}

func GuildSearch(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildSearch(buffer, client)
}

func GuildJoinMissionCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildJoinMissionCommandResponse(buffer, client)
}

func GuildRefreshMissionCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildRefreshMissionCommandResponse(buffer, client)
}

func GuildShopPurchase(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildShopPurchase(buffer, client)
}

func GuildActiveEventCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildActiveEventCommandResponse(buffer, client)
}

func ModifyGuildInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.ModifyGuildInfo(buffer, client)
}

func GuildImpeach(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildImpeach(buffer, client)
}

func GuildFire(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildFire(buffer, client)
}

func GuildQuit(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildQuit(buffer, client)
}

func GuildDissolve(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildDissolve(buffer, client)
}

func GuildSendMessage(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GuildSendMessage(buffer, client)
}

func GetGuildShop(buffer *[]byte, client *connection.Client) (int, int, error) {
	return guild.GetGuildShop(buffer, client)
}
