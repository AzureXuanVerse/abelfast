package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestGetMailTitleListSkipsMissingIDs(t *testing.T) {
	client := setupPlayerUpdateTest(t)

	mail := orm.Mail{ReceiverID: client.Commander.CommanderID, Title: "Title", Body: "Body"}
	sender := "HQ"
	mail.CustomSender = &sender
	if err := mail.Create(); err != nil {
		t.Fatalf("create mail: %v", err)
	}
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_30014{IdList: []uint32{mail.ID, 999999}})
	if _, _, err := GetMailTitleList(&payload, client); err != nil {
		t.Fatalf("GetMailTitleList failed: %v", err)
	}

	response := &protobuf.SC_30015{}
	decodeLoveLetterPacketMessage(t, client, 30015, response)
	if len(response.GetMailTitleList()) != 1 {
		t.Fatalf("expected 1 title, got %d", len(response.GetMailTitleList()))
	}
	if response.GetMailTitleList()[0].GetTitle() != "Title||HQ" {
		t.Fatalf("unexpected title: %q", response.GetMailTitleList()[0].GetTitle())
	}
}

func TestGetMailTitleListDecodeFailure(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	garbage := []byte{0xff, 0x01}
	if _, packetID, err := GetMailTitleList(&garbage, client); err == nil || packetID != 30015 {
		t.Fatalf("expected decode error targeting 30015, got packet=%d err=%v", packetID, err)
	}
}

func TestCheckLoveLetterItemMailReturnsSortedYearsWithSentinel(t *testing.T) {
	client := setupLoveLetterTestClient(t)
	seedConfigEntry(t, loveLetterLegacyTemplateCategory, "51002", `{"id":51002,"ship_group_id":10000,"year":2019}`)

	payload := marshalPacketRequest(t, &protobuf.CS_30016{ItemId: proto.Uint32(51002), Groupid: proto.Uint32(10000)})
	if _, _, err := CheckLoveLetterItemMail(&payload, client); err != nil {
		t.Fatalf("CheckLoveLetterItemMail failed: %v", err)
	}

	response := &protobuf.SC_30017{}
	decodeLoveLetterPacketMessage(t, client, 30017, response)
	years := response.GetYears()
	if len(years) != 2 || years[0] != 0 || years[1] != 2019 {
		t.Fatalf("unexpected years: %+v", years)
	}
}

func TestCheckLoveLetterItemMailDecodeFailure(t *testing.T) {
	client := setupLoveLetterTestClient(t)
	garbage := []byte{0xff, 0x01}
	if _, packetID, err := CheckLoveLetterItemMail(&garbage, client); err == nil || packetID != 30016 {
		t.Fatalf("expected decode error targeting 30016, got packet=%d err=%v", packetID, err)
	}
}

func TestRepairLoveLetterItemMailSuccess(t *testing.T) {
	client := setupLoveLetterTestClient(t)
	seedTestItemTemplateForMailChunk(t, 41002)
	seedTestItemTemplateForMailChunk(t, 2018001)
	if err := client.Commander.AddItem(41002, 1); err != nil {
		t.Fatalf("seed input item: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_30018{ItemId: proto.Uint32(41002), Groupid: proto.Uint32(10000), Year: proto.Uint32(2018)})
	if _, _, err := RepairLoveLetterItemMail(&payload, client); err != nil {
		t.Fatalf("RepairLoveLetterItemMail failed: %v", err)
	}

	response := &protobuf.SC_30019{}
	decodeLoveLetterPacketMessage(t, client, 30019, response)
	if response.GetRet() != 0 {
		t.Fatalf("expected success, got %d", response.GetRet())
	}
	if len(response.GetDropList()) != 1 || response.GetDropList()[0].GetId() != 2018001 {
		t.Fatalf("unexpected drop list: %+v", response.GetDropList())
	}
	if client.Commander.GetItemCount(41002) != 0 {
		t.Fatalf("expected source item consumed")
	}
	if client.Commander.GetItemCount(2018001) != 1 {
		t.Fatalf("expected repaired love letter item granted")
	}
}

func TestRepairLoveLetterItemMailAmbiguousYearFails(t *testing.T) {
	client := setupLoveLetterTestClient(t)
	seedConfigEntry(t, loveLetterContentTemplateCategory, "2019001", `{"id":2019001,"ship_group":10000,"year":2019,"love_item":[41002],"content":""}`)
	seedConfigEntry(t, loveLetterLegacyTemplateCategory, "41002", `{"id":41002,"ship_group_id":10000,"year":2018}`)
	seedTestItemTemplateForMailChunk(t, 41002)
	if err := client.Commander.AddItem(41002, 1); err != nil {
		t.Fatalf("seed input item: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_30018{ItemId: proto.Uint32(41002), Groupid: proto.Uint32(10000), Year: proto.Uint32(0)})
	if _, _, err := RepairLoveLetterItemMail(&payload, client); err != nil {
		t.Fatalf("RepairLoveLetterItemMail failed: %v", err)
	}

	response := &protobuf.SC_30019{}
	decodeLoveLetterPacketMessage(t, client, 30019, response)
	if response.GetRet() != loveLetterRepairResultInvalid {
		t.Fatalf("expected invalid ret code, got %d", response.GetRet())
	}
	if client.Commander.GetItemCount(41002) != 1 {
		t.Fatalf("expected no item consumption on failure")
	}
}

func TestWithdrawMailStoreroomResourcesSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedConfigEntry(t, "ShareCfg/gameset.json", "max_oil", `{"key_value":25000,"description":""}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "max_gold", `{"key_value":600000,"description":""}`)
	if err := client.Commander.SetResource(mailStoreroomStoredOilResource, 500); err != nil {
		t.Fatalf("seed stored oil: %v", err)
	}
	if err := client.Commander.SetResource(mailStoreroomStoredGoldResource, 500); err != nil {
		t.Fatalf("seed stored gold: %v", err)
	}

	startActiveOil := queryAnswerTestInt64(t, "SELECT COALESCE((SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2), 0)", int64(client.Commander.CommanderID), int64(mailStoreroomOilResourceID))
	startActiveGold := queryAnswerTestInt64(t, "SELECT COALESCE((SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2), 0)", int64(client.Commander.CommanderID), int64(mailStoreroomGoldResourceID))

	payload := marshalPacketRequest(t, &protobuf.CS_30012{Oil: proto.Uint32(200), Gold: proto.Uint32(100)})
	if _, _, err := WithdrawMailStoreroomResources(&payload, client); err != nil {
		t.Fatalf("WithdrawMailStoreroomResources failed: %v", err)
	}

	response := &protobuf.SC_30013{}
	decodeLoveLetterPacketMessage(t, client, 30013, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success, got %d", response.GetResult())
	}
	if client.Commander.GetResourceCount(mailStoreroomStoredOilResource) != 300 {
		t.Fatalf("expected stored oil decremented")
	}
	if client.Commander.GetResourceCount(mailStoreroomStoredGoldResource) != 400 {
		t.Fatalf("expected stored gold decremented")
	}
	activeOil := queryAnswerTestInt64(t, "SELECT COALESCE((SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2), 0)", int64(client.Commander.CommanderID), int64(mailStoreroomOilResourceID))
	if activeOil != startActiveOil+200 {
		t.Fatalf("expected active oil incremented")
	}
	activeGold := queryAnswerTestInt64(t, "SELECT COALESCE((SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2), 0)", int64(client.Commander.CommanderID), int64(mailStoreroomGoldResourceID))
	if activeGold != startActiveGold+100 {
		t.Fatalf("expected active gold incremented")
	}
}

func TestWithdrawMailStoreroomResourcesCapFailureDoesNotMutate(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedConfigEntry(t, "ShareCfg/gameset.json", "max_oil", `{"key_value":25000,"description":""}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "max_gold", `{"key_value":600000,"description":""}`)
	if err := client.Commander.SetResource(mailStoreroomOilResourceID, 24980); err != nil {
		t.Fatalf("seed active oil: %v", err)
	}
	if err := client.Commander.SetResource(mailStoreroomGoldResourceID, 599980); err != nil {
		t.Fatalf("seed active gold: %v", err)
	}
	if err := client.Commander.SetResource(mailStoreroomStoredOilResource, 500); err != nil {
		t.Fatalf("seed stored oil: %v", err)
	}
	if err := client.Commander.SetResource(mailStoreroomStoredGoldResource, 500); err != nil {
		t.Fatalf("seed stored gold: %v", err)
	}
	payload := marshalPacketRequest(t, &protobuf.CS_30012{Oil: proto.Uint32(100), Gold: proto.Uint32(100)})
	if _, _, err := WithdrawMailStoreroomResources(&payload, client); err != nil {
		t.Fatalf("WithdrawMailStoreroomResources failed: %v", err)
	}

	response := &protobuf.SC_30013{}
	decodeLoveLetterPacketMessage(t, client, 30013, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected cap check failure")
	}
	if client.Commander.GetResourceCount(mailStoreroomStoredOilResource) != 500 ||
		client.Commander.GetResourceCount(mailStoreroomStoredGoldResource) != 500 {
		t.Fatalf("expected stored resources unchanged")
	}
}

func TestWithdrawMailStoreroomResourcesSingleResourceWithoutPeerRow(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedConfigEntry(t, "ShareCfg/gameset.json", "max_oil", `{"key_value":25000,"description":""}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "max_gold", `{"key_value":600000,"description":""}`)
	if err := client.Commander.SetResource(mailStoreroomStoredOilResource, 500); err != nil {
		t.Fatalf("seed stored oil: %v", err)
	}
	startActiveOil := queryAnswerTestInt64(t, "SELECT COALESCE((SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2), 0)", int64(client.Commander.CommanderID), int64(mailStoreroomOilResourceID))

	execAnswerTestSQLT(t, "DELETE FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(mailStoreroomStoredGoldResource))

	payload := marshalPacketRequest(t, &protobuf.CS_30012{Oil: proto.Uint32(200), Gold: proto.Uint32(0)})
	if _, _, err := WithdrawMailStoreroomResources(&payload, client); err != nil {
		t.Fatalf("WithdrawMailStoreroomResources failed: %v", err)
	}

	response := &protobuf.SC_30013{}
	decodeLoveLetterPacketMessage(t, client, 30013, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success, got %d", response.GetResult())
	}
	if client.Commander.GetResourceCount(mailStoreroomStoredOilResource) != 300 {
		t.Fatalf("expected stored oil decremented")
	}
	activeOil := queryAnswerTestInt64(t, "SELECT COALESCE((SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2), 0)", int64(client.Commander.CommanderID), int64(mailStoreroomOilResourceID))
	if activeOil != startActiveOil+200 {
		t.Fatalf("expected active oil incremented")
	}
}

func TestExtendMailStoreroomCapacityPersistsLevel(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedConfigEntry(t, mailStoreroomConfigEntryCategory, "1", `{"id":1,"level":1,"upgrade_gem":0,"upgrade_gold":100,"oil_store":600000,"gold_store":1800000}`)
	seedConfigEntry(t, mailStoreroomConfigEntryCategory, "2", `{"id":2,"level":2,"upgrade_gem":0,"upgrade_gold":200,"oil_store":700000,"gold_store":1900000}`)
	if err := client.Commander.SetResource(mailStoreroomGoldResourceID, 500); err != nil {
		t.Fatalf("seed gold: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_30010{Arg: proto.Uint32(mailStoreroomGoldResourceID)})
	if _, _, err := ExtendMailStoreroomCapacity(&payload, client); err != nil {
		t.Fatalf("ExtendMailStoreroomCapacity failed: %v", err)
	}

	response := &protobuf.SC_30011{}
	decodeLoveLetterPacketMessage(t, client, 30011, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success, got %d", response.GetResult())
	}
	if client.Commander.MailStoreroomLv != 2 {
		t.Fatalf("expected in-memory level 2, got %d", client.Commander.MailStoreroomLv)
	}
	reloaded := orm.Commander{CommanderID: client.Commander.CommanderID}
	if err := reloaded.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	if reloaded.MailStoreroomLv != 2 {
		t.Fatalf("expected persisted level 2, got %d", reloaded.MailStoreroomLv)
	}
}

func TestExtendMailStoreroomCapacityDecodeFailure(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	garbage := []byte{0xff, 0x01}
	if _, packetID, err := ExtendMailStoreroomCapacity(&garbage, client); err == nil || packetID != 30010 {
		t.Fatalf("expected decode error targeting 30010, got packet=%d err=%v", packetID, err)
	}
}

func seedTestItemTemplateForMailChunk(t *testing.T, itemID uint32) {
	t.Helper()
	execAnswerTestSQLT(t,
		"INSERT INTO items (id, name, rarity, shop_id, type, virtual_type) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING",
		int64(itemID),
		"Mail Chunk Item",
		int64(1),
		int64(0),
		int64(1),
		int64(0),
	)
}
