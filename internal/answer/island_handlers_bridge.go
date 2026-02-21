package answer

import (
	"github.com/ggmolly/belfast/internal/answer/island"
	"github.com/ggmolly/belfast/internal/connection"
)

const islandNodeResultSuccess = uint32(0)

func StartIslandHandPlant(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.StartIslandHandPlant(buffer, client)
}

func IslandClaimHandPlantAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandClaimHandPlantAward(buffer, client)
}

func IslandCollectSlot(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandCollectSlot(buffer, client)
}

func IslandGetDelegationAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandGetDelegationAward(buffer, client)
}

func IslandFinishDelegation(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandFinishDelegation(buffer, client)
}

func IslandStartDelegation(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandStartDelegation(buffer, client)
}

func IslandAddDelegation(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandAddDelegation(buffer, client)
}

func IslandUseDelegationTicket(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandUseDelegationTicket(buffer, client)
}

func IslandRemoveExpiredTicket(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandRemoveExpiredTicket(buffer, client)
}

func IslandUseTicket(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandUseTicket(buffer, client)
}

func IslandCloseRestaurant(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandCloseRestaurant(buffer, client)
}

func IslandOpenRestaurant(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandOpenRestaurant(buffer, client)
}

func IslandUpgrade(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandUpgrade(buffer, client)
}

func IslandSetAccessAuthority(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetAccessAuthority(buffer, client)
}

func IslandSetName(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetName(buffer, client)
}

func IslandTransferOverflowItems(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandTransferOverflowItems(buffer, client)
}

func IslandUpgradeInventory(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandUpgradeInventory(buffer, client)
}

func IslandSellOrConvertItems(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSellOrConvertItems(buffer, client)
}

func IslandShopGetData(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandShopGetData(buffer, client)
}

func IslandShopPurchase(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandShopPurchase(buffer, client)
}

func IslandShipOrderOperate(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandShipOrderOperate(buffer, client)
}

func IslandShipOrderSubmit(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandShipOrderSubmit(buffer, client)
}

func HandleIslandShipOrderRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandShipOrderRefresh(buffer, client)
}

func HandleIslandShipBreakout(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandShipBreakout(buffer, client)
}

func HandleIslandShipAttrLimitUnlock(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandShipAttrLimitUnlock(buffer, client)
}

func HandleIslandShipAttrUpgrade(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandShipAttrUpgrade(buffer, client)
}

func HandleIslandUseShipExpBook(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandUseShipExpBook(buffer, client)
}

func HandleIslandInviteShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandInviteShip(buffer, client)
}

func HandleIslandShipSkillUpgrade(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandShipSkillUpgrade(buffer, client)
}

func HandleIslandGiveGift(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandGiveGift(buffer, client)
}

func HandleIslandChangeDress(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandChangeDress(buffer, client)
}

func IslandBuyRoleSkinColor(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandBuyRoleSkinColor(buffer, client)
}

func IslandSetRoleDressRead(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetRoleDressRead(buffer, client)
}

func IslandChangeCommanderDress(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandChangeCommanderDress(buffer, client)
}

func IslandBuyDressColor(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandBuyDressColor(buffer, client)
}

func IslandGetNpcActionAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandGetNpcActionAward(buffer, client)
}

func IslandFollowerOp(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandFollowerOp(buffer, client)
}

func HandleIslandStopHandPlantHalfway(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandStopHandPlantHalfway(buffer, client)
}

func IslandUnlockTech(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandUnlockTech(buffer, client)
}

func IslandFinishTechImmediate(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandFinishTechImmediate(buffer, client)
}

func IslandClaimProsperityReward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandClaimProsperityReward(buffer, client)
}

func IslandClaimAchievementAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandClaimAchievementAward(buffer, client)
}

func IslandSyncAchievementProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSyncAchievementProgress(buffer, client)
}

func IslandClaimSeasonPTReward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandClaimSeasonPTReward(buffer, client)
}

func IslandOrderSync(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandOrderSync(buffer, client)
}

func IslandRandomTaskRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandRandomTaskRefresh(buffer, client)
}

func IslandShopPlayerRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandShopPlayerRefresh(buffer, client)
}

func IslandUseItem(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandUseItem(buffer, client)
}

func IslandGoFishing(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandGoFishing(buffer, client)
}

func IslandFishingResult(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandFishingResult(buffer, client)
}

func IslandExchangeLure(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandExchangeLure(buffer, client)
}

func IslandExchangeItem(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandExchangeItem(buffer, client)
}

func IslandGetData(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandGetData(buffer, client)
}

func IslandTradeOp(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandTradeOp(buffer, client)
}

func IslandGetFriendTradeRank(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandGetFriendTradeRank(buffer, client)
}

func IslandSetAccessTypeLegacy(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetAccessTypeLegacy(buffer, client)
}

func IslandAccessOp(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandAccessOp(buffer, client)
}

func IslandUpgradeAgora(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandUpgradeAgora(buffer, client)
}

func IslandSaveAgoraPlacement(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSaveAgoraPlacement(buffer, client)
}

func IslandSignInGiftClaim(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSignInGiftClaim(buffer, client)
}

func IslandGetCardData(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandGetCardData(buffer, client)
}

func IslandSetCardPhoto(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetCardPhoto(buffer, client)
}

func IslandSetCardWord(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetCardWord(buffer, client)
}

func IslandSettingFlag(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSettingFlag(buffer, client)
}

func IslandGiveCardLike(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandGiveCardLike(buffer, client)
}

func IslandGiveCardLabel(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandGiveCardLabel(buffer, client)
}

func IslandSetCardAchievements(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetCardAchievements(buffer, client)
}

func IslandBookUnlock(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandBookUnlock(buffer, client)
}

func IslandBookCollectPoint(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandBookCollectPoint(buffer, client)
}

func IslandBookPointAwardClaim(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandBookPointAwardClaim(buffer, client)
}

func IslandSetCommanderDressRead(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetCommanderDressRead(buffer, client)
}

func IslandRefreshInviteCode(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandRefreshInviteCode(buffer, client)
}

func IslandSetTraceTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetTraceTask(buffer, client)
}

func IslandAcceptTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandAcceptTask(buffer, client)
}

func IslandUpdateTaskProgress(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandUpdateTaskProgress(buffer, client)
}

func IslandSubmitTask(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSubmitTask(buffer, client)
}

func IslandSubmitTaskOneStep(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSubmitTaskOneStep(buffer, client)
}

func IslandEnter(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandEnter(buffer, client)
}

func IslandExit(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandExit(buffer, client)
}

func HandleIslandQueuePoll(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandQueuePoll(buffer, client)
}

func IslandSyncControl(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSyncControl(buffer, client)
}

func IslandSyncData(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSyncData(buffer, client)
}

func IslandEnterMap(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandEnterMap(buffer, client)
}

func IslandHeartbeat(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandHeartbeat(buffer, client)
}

func HandleIslandRecordLastPosition(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandRecordLastPosition(buffer, client)
}

func HandleIslandReconnect(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandReconnect(buffer, client)
}

func IslandAnimationOp(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandAnimationOp(buffer, client)
}

func IslandSignInInvitation(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSignInInvitation(buffer, client)
}

func IslandTradeInvitation(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandTradeInvitation(buffer, client)
}

func HandleIslandGetGiftTag(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandGetGiftTag(buffer, client)
}

func IslandSendChat(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSendChat(buffer, client)
}

func IslandUpdateIllustration(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandUpdateIllustration(buffer, client)
}

func IslandReplaceOrder(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandReplaceOrder(buffer, client)
}

func IslandSubmitCommonOrder(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSubmitCommonOrder(buffer, client)
}

func IslandSetOrderTendency(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSetOrderTendency(buffer, client)
}

func IslandSubmitFirmOrder(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSubmitFirmOrder(buffer, client)
}

func IslandSubmitUrgencyOrder(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandSubmitUrgencyOrder(buffer, client)
}

func IslandClaimOrderFavorReward(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandClaimOrderFavorReward(buffer, client)
}

func IslandExchangeShipOrderDelegate(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandExchangeShipOrderDelegate(buffer, client)
}

func HandleIslandWildGatherCollect(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandWildGatherCollect(buffer, client)
}

func HandleIslandWildCollectFragment(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandWildCollectFragment(buffer, client)
}

func HandleIslandWildCollectFragmentSign(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandWildCollectFragmentSign(buffer, client)
}

func HandleIslandWildGatherSign(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandWildGatherSign(buffer, client)
}

func HandleIslandCollectionComplete(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.HandleIslandCollectionComplete(buffer, client)
}

func IslandRequestNodeList(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.IslandRequestNodeList(buffer, client)
}

func SaveIslandAgoraTheme(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.SaveIslandAgoraTheme(buffer, client)
}

func ListIslandAgoraThemes(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.ListIslandAgoraThemes(buffer, client)
}

func DeleteIslandAgoraTheme(buffer *[]byte, client *connection.Client) (int, int, error) {
	return island.DeleteIslandAgoraTheme(buffer, client)
}
