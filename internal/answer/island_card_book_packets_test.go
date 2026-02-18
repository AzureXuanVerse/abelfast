package answer

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestIslandCardSettingsAndAchievementFlow(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandCardState{})
	clearTable(t, &orm.IslandAchievementState{})

	seedConfigEntry(t, islandSetCategory, "island_card_photo_default", `{"key":"island_card_photo_default","key_value_int":4001}`)
	seedConfigEntry(t, islandCardDIYCategory, "4001", `{"id":4001}`)
	seedConfigEntry(t, islandAchievementCategory, "1", `{"id":1,"group":1,"stage":1,"target_type":1,"target_value1":0}`)
	seedConfigEntry(t, islandAchievementCategory, "2", `{"id":2,"group":1,"stage":2,"target_type":1,"target_value1":0}`)
	seedConfigEntry(t, islandAchievementCategory, "21", `{"id":21,"group":2,"stage":1,"target_type":1,"target_value1":0}`)

	if err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state := orm.NewIslandAchievementState(client.Commander.CommanderID)
		state.FinishList = []uint32{1, 2, 21}
		return orm.SaveIslandAchievementStateTx(context.Background(), tx, state)
	}); err != nil {
		t.Fatalf("seed achievement state: %v", err)
	}

	invalidWordPayload, _ := proto.Marshal(&protobuf.CS_21330{VisitWord: proto.String("bad")})
	if _, _, err := IslandSetCardWord(&invalidWordPayload, client); err != nil {
		t.Fatalf("set card word invalid call failed: %v", err)
	}
	var invalidWordResp protobuf.SC_21331
	decodePacketAt(t, client, 0, 21331, &invalidWordResp)
	if invalidWordResp.GetResult() == 0 {
		t.Fatalf("expected invalid card word to fail")
	}

	client.Buffer.Reset()
	wordPayload, _ := proto.Marshal(&protobuf.CS_21330{VisitWord: proto.String("hello island")})
	if _, _, err := IslandSetCardWord(&wordPayload, client); err != nil {
		t.Fatalf("set card word failed: %v", err)
	}
	var wordResp protobuf.SC_21331
	decodePacketAt(t, client, 0, 21331, &wordResp)
	if wordResp.GetResult() != 0 {
		t.Fatalf("expected card word success, got %d", wordResp.GetResult())
	}

	client.Buffer.Reset()
	flagPayload, _ := proto.Marshal(&protobuf.CS_21332{FlagList: []*protobuf.PB_SET_FLAG{{Type: proto.Uint32(islandCardFlagLabel), Flag: proto.Uint32(0)}}})
	if _, _, err := IslandSettingFlag(&flagPayload, client); err != nil {
		t.Fatalf("set card flags failed: %v", err)
	}
	var flagResp protobuf.SC_21333
	decodePacketAt(t, client, 0, 21333, &flagResp)
	if flagResp.GetResult() != 0 {
		t.Fatalf("expected settings success, got %d", flagResp.GetResult())
	}

	client.Buffer.Reset()
	achvPayload, _ := proto.Marshal(&protobuf.CS_21338{GroupList: []uint32{1, 2}})
	if _, _, err := IslandSetCardAchievements(&achvPayload, client); err != nil {
		t.Fatalf("set card achievements failed: %v", err)
	}
	var achvResp protobuf.SC_21339
	decodePacketAt(t, client, 0, 21339, &achvResp)
	if achvResp.GetResult() != 0 {
		t.Fatalf("expected achievement selection success, got %d", achvResp.GetResult())
	}

	client.Buffer.Reset()
	fetchPayload, _ := proto.Marshal(&protobuf.CS_21326{UserId: proto.Uint32(client.Commander.CommanderID)})
	if _, _, err := IslandGetCardData(&fetchPayload, client); err != nil {
		t.Fatalf("get card data failed: %v", err)
	}
	var fetchResp protobuf.SC_21327
	decodePacketAt(t, client, 0, 21327, &fetchResp)
	if fetchResp.GetLabelViewFlag() != 0 {
		t.Fatalf("expected label view flag 0, got %d", fetchResp.GetLabelViewFlag())
	}
	if len(fetchResp.GetAchieveList()) != 2 || fetchResp.GetAchieveList()[0] != 2 {
		t.Fatalf("unexpected achieve list: %v", fetchResp.GetAchieveList())
	}
	if fetchResp.GetPicture() != "4001" {
		t.Fatalf("expected default picture 4001, got %q", fetchResp.GetPicture())
	}
}

func TestIslandCardLikeAndLabelFlow(t *testing.T) {
	sender := setupHandlerCommander(t)
	receiver := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandCardState{})

	seedConfigEntry(t, islandCardLabelCategory, "1", `{"id":1}`)

	likePayload, _ := proto.Marshal(&protobuf.CS_21334{UserId: proto.Uint32(receiver.Commander.CommanderID)})
	if _, _, err := IslandGiveCardLike(&likePayload, sender); err != nil {
		t.Fatalf("give like failed: %v", err)
	}
	var likeResp protobuf.SC_21335
	decodePacketAt(t, sender, 0, 21335, &likeResp)
	if likeResp.GetResult() != 0 {
		t.Fatalf("expected like success, got %d", likeResp.GetResult())
	}

	sender.Buffer.Reset()
	if _, _, err := IslandGiveCardLike(&likePayload, sender); err != nil {
		t.Fatalf("duplicate like call failed: %v", err)
	}
	decodePacketAt(t, sender, 0, 21335, &likeResp)
	if likeResp.GetResult() == 0 {
		t.Fatalf("expected duplicate like failure")
	}

	sender.Buffer.Reset()
	labelPayload, _ := proto.Marshal(&protobuf.CS_21336{UserId: proto.Uint32(receiver.Commander.CommanderID), LabelId: proto.Uint32(1)})
	if _, _, err := IslandGiveCardLabel(&labelPayload, sender); err != nil {
		t.Fatalf("give label failed: %v", err)
	}
	var labelResp protobuf.SC_21337
	decodePacketAt(t, sender, 0, 21337, &labelResp)
	if labelResp.GetResult() != 0 {
		t.Fatalf("expected label success, got %d", labelResp.GetResult())
	}

	sender.Buffer.Reset()
	if _, _, err := IslandGiveCardLabel(&labelPayload, sender); err != nil {
		t.Fatalf("duplicate label call failed: %v", err)
	}
	decodePacketAt(t, sender, 0, 21337, &labelResp)
	if labelResp.GetResult() == 0 {
		t.Fatalf("expected duplicate label failure")
	}

	sender.Buffer.Reset()
	fetchPayload, _ := proto.Marshal(&protobuf.CS_21326{UserId: proto.Uint32(receiver.Commander.CommanderID)})
	if _, _, err := IslandGetCardData(&fetchPayload, sender); err != nil {
		t.Fatalf("fetch card after social actions failed: %v", err)
	}
	var cardResp protobuf.SC_21327
	decodePacketAt(t, sender, 0, 21327, &cardResp)
	if cardResp.GetGoodFlag() != 1 || cardResp.GetLabelFlag() != 1 {
		t.Fatalf("expected like/label marks set, got good=%d label=%d", cardResp.GetGoodFlag(), cardResp.GetLabelFlag())
	}
	if cardResp.GetGoodNum() != 1 || len(cardResp.GetLabelList()) != 1 || cardResp.GetLabelList()[0].GetNum() != 1 {
		t.Fatalf("unexpected card social counters: good=%d labels=%v", cardResp.GetGoodNum(), cardResp.GetLabelList())
	}
}

func TestIslandCardGetDataMissingTargetReturnsEmptyPayload(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	seedConfigEntry(t, islandSetCategory, "island_card_photo_default", `{"key":"island_card_photo_default","key_value_int":4001}`)

	payload, _ := proto.Marshal(&protobuf.CS_21326{UserId: proto.Uint32(4294967295)})
	if _, _, err := IslandGetCardData(&payload, client); err != nil {
		t.Fatalf("get card data missing target failed: %v", err)
	}

	var response protobuf.SC_21327
	decodePacketAt(t, client, 0, 21327, &response)
	if response.GetName() != "" || response.GetPicture() != "4001" || response.GetLv() != 0 {
		t.Fatalf("unexpected missing-target response: %+v", response)
	}
}

func TestIslandGiveCardLikeRespectsSocialFlag(t *testing.T) {
	sender := setupHandlerCommander(t)
	receiver := setupHandlerCommander(t)
	clearTable(t, &orm.IslandCardState{})

	if err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state := orm.NewIslandCardState(receiver.Commander.CommanderID)
		state.SocialFlag = 0
		return orm.SaveIslandCardStateTx(context.Background(), tx, state)
	}); err != nil {
		t.Fatalf("seed receiver social flag: %v", err)
	}

	likePayload, _ := proto.Marshal(&protobuf.CS_21334{UserId: proto.Uint32(receiver.Commander.CommanderID)})
	if _, _, err := IslandGiveCardLike(&likePayload, sender); err != nil {
		t.Fatalf("give like with social off failed: %v", err)
	}

	var likeResp protobuf.SC_21335
	decodePacketAt(t, sender, 0, 21335, &likeResp)
	if likeResp.GetResult() == 0 {
		t.Fatalf("expected like rejection when social flag disabled")
	}

	liked, err := orm.HasIslandCardLike(sender.Commander.CommanderID, receiver.Commander.CommanderID)
	if err != nil {
		t.Fatalf("check like relation: %v", err)
	}
	if liked {
		t.Fatalf("expected no persisted like when social flag disabled")
	}
}

func TestIslandGetCardDataIncrementsVisitCountForVisitor(t *testing.T) {
	viewer := setupHandlerCommander(t)
	target := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandCardState{})
	seedConfigEntry(t, islandSetCategory, "island_card_photo_default", `{"key":"island_card_photo_default","key_value_int":4001}`)

	payload, _ := proto.Marshal(&protobuf.CS_21326{UserId: proto.Uint32(target.Commander.CommanderID)})
	if _, _, err := IslandGetCardData(&payload, viewer); err != nil {
		t.Fatalf("first get card data failed: %v", err)
	}
	var firstResp protobuf.SC_21327
	decodePacketAt(t, viewer, 0, 21327, &firstResp)
	if firstResp.GetVisitNum() != 1 {
		t.Fatalf("expected first visit count 1, got %d", firstResp.GetVisitNum())
	}

	viewer.Buffer.Reset()
	if _, _, err := IslandGetCardData(&payload, viewer); err != nil {
		t.Fatalf("second get card data failed: %v", err)
	}
	var secondResp protobuf.SC_21327
	decodePacketAt(t, viewer, 0, 21327, &secondResp)
	if secondResp.GetVisitNum() != 2 {
		t.Fatalf("expected second visit count 2, got %d", secondResp.GetVisitNum())
	}
}

func TestIslandBookUnlockCollectAndAwardFlow(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandBookState{})

	seedConfigEntry(t, islandIllustratedGuideCategory, "1", `{"id":1,"type":1,"collect_add":20,"collect_upgrade":[[50,50]],"collect_star":[[2,30]],"award_unlock":[[2,20001,1]]}`)
	seedConfigEntry(t, islandCollectionRewardCategory, "1", `{"id":1,"type":1,"need_exp":20,"award_display":[2,20001,2]}`)

	unlockPayload, _ := proto.Marshal(&protobuf.CS_21343{BookIds: []uint32{1}})
	if _, _, err := IslandBookUnlock(&unlockPayload, client); err != nil {
		t.Fatalf("book unlock failed: %v", err)
	}
	var unlockResp protobuf.SC_21344
	decodePacketAt(t, client, 0, 21344, &unlockResp)
	if unlockResp.GetResult() != 0 {
		t.Fatalf("expected unlock success, got %d", unlockResp.GetResult())
	}

	client.Buffer.Reset()
	collectPayload, _ := proto.Marshal(&protobuf.CS_21345{BookIds: []uint32{1}})
	if _, _, err := IslandBookCollectPoint(&collectPayload, client); err != nil {
		t.Fatalf("collect point failed: %v", err)
	}
	var collectResp protobuf.SC_21346
	decodePacketAt(t, client, 0, 21346, &collectResp)
	if collectResp.GetResult() != 0 || len(collectResp.GetCollectList()) != 1 {
		t.Fatalf("unexpected collect response: result=%d len=%d", collectResp.GetResult(), len(collectResp.GetCollectList()))
	}

	client.Buffer.Reset()
	awardPayload, _ := proto.Marshal(&protobuf.CS_21347{Lv: proto.Uint32(1)})
	if _, _, err := IslandBookPointAwardClaim(&awardPayload, client); err != nil {
		t.Fatalf("point award claim failed: %v", err)
	}
	var awardResp protobuf.SC_21348
	decodePacketAt(t, client, 0, 21348, &awardResp)
	if awardResp.GetResult() != 0 {
		t.Fatalf("expected point award success, got %d", awardResp.GetResult())
	}

	client.Buffer.Reset()
	getDataPayload, _ := proto.Marshal(&protobuf.CS_21200{IslandId: proto.Uint32(client.Commander.CommanderID)})
	if _, _, err := IslandGetData(&getDataPayload, client); err != nil {
		t.Fatalf("island get data failed: %v", err)
	}
	var getDataResp protobuf.SC_21201
	decodePacketAt(t, client, 0, 21201, &getDataResp)
	viewBook := getDataResp.GetIsland().GetPrivateData().GetViewBook()
	if len(viewBook.GetBookList()) != 1 || len(viewBook.GetBookAwards()) != 1 || len(viewBook.GetBookCollects()) != 1 {
		t.Fatalf("unexpected projected book state: %+v", viewBook)
	}
}
