package answer

import (
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupGameRoomTestClient(t *testing.T, commanderID uint32, gold uint32, gameCoin uint32, gameTicket uint32) *connection.Client {
	t.Helper()
	t.Setenv("MODE", "test")
	orm.InitDatabase()

	clearTable(t, &orm.GameRoomScore{})
	clearTable(t, &orm.GameRoomState{})
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.Resource{})
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.Commander{})

	if err := orm.CreateCommanderRoot(commanderID, commanderID, fmt.Sprintf("Game Room %d", commanderID), 0, 0); err != nil {
		t.Fatalf("create commander root: %v", err)
	}
	for _, resource := range []struct {
		id   uint32
		name string
	}{
		{id: 1, name: "gold"},
		{id: 11, name: "gamecoin"},
		{id: 12, name: "gameticket"},
	} {
		execAnswerTestSQLT(t, "INSERT INTO resources (id, item_id, name) VALUES ($1, $2, $3)", int64(resource.id), int64(0), resource.name)
	}
	for _, owned := range []struct {
		id     uint32
		amount uint32
	}{
		{id: 1, amount: gold},
		{id: 11, amount: gameCoin},
		{id: 12, amount: gameTicket},
	} {
		execAnswerTestSQLT(t, "INSERT INTO owned_resources (commander_id, resource_id, amount) VALUES ($1, $2, $3)", int64(commanderID), int64(owned.id), int64(owned.amount))
	}

	seedGameRoomConfigData(t)

	client := &connection.Client{Commander: &orm.Commander{CommanderID: commanderID}}
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}
	return client
}

func seedGameRoomConfigData(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, "ShareCfg/gameset.json", "game_coin_initial", `{"description":"","key_value":10}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "game_coin_max", `{"description":"","key_value":40}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "game_coin_gold", `{"description":[[0,800],[5,1200],[10,2000]],"key_value":0}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "game_ticket_month", `{"description":"","key_value":10000}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "game_room_remax", `{"description":"","key_value":50000}`)
	seedConfigEntry(t, "ShareCfg/game_room_template.json", "1", `{"id":1,"add_base":200,"add_num":[[31,1.3],[21,1.2],[11,1.1],[6,1],[0,0.9]],"add_type":12,"coin_max":5}`)
}

func TestGameRoomWeeklyCoinClaimAndProjection(t *testing.T) {
	client := setupGameRoomTestClient(t, 9811, 9999, 5, 0)
	request := &protobuf.CS_26122{Type: proto.Uint32(0)}
	buffer, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := GameRoomWeeklyCoinClaim(&buffer, client); err != nil {
		t.Fatalf("weekly coin claim failed: %v", err)
	}
	resp := &protobuf.SC_26123{}
	offset := decodePacketAt(t, client, 0, 26123, resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected success result")
	}
	coin := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(11))
	if coin != 15 {
		t.Fatalf("expected coin to increase to 15, got %d", coin)
	}

	if _, _, err := GameRoomWeeklyCoinClaim(&buffer, client); err != nil {
		t.Fatalf("second weekly coin claim failed: %v", err)
	}
	second := &protobuf.SC_26123{}
	decodePacketAt(t, client, offset, 26123, second)
	if second.GetResult() == 0 {
		t.Fatalf("expected second claim to fail")
	}

	projectionClient := &connection.Client{Commander: &orm.Commander{CommanderID: client.Commander.CommanderID}}
	if _, _, err := EventData(&[]byte{}, projectionClient); err != nil {
		t.Fatalf("event data failed: %v", err)
	}
	projection := &protobuf.SC_26120{}
	decodePacketAt(t, projectionClient, 0, 26120, projection)
	if projection.GetWeeklyFree() != 1 {
		t.Fatalf("expected weekly_free to be claimed")
	}
}

func TestGameRoomExchangeCoinClampAndProjection(t *testing.T) {
	client := setupGameRoomTestClient(t, 9812, 10000, 38, 0)
	now := time.Now().UTC()
	weekStart := orm.CurrentWeeklyResetUnix(now)
	monthKey := uint32(now.Year()*100 + int(now.Month()))
	execAnswerTestSQLT(t, `
INSERT INTO game_room_states (commander_id, week_start_unix, weekly_claimed, pay_coin_count, first_enter_claimed, month_key, monthly_ticket)
VALUES ($1, $2, false, $3, false, $4, 0)
`, int64(client.Commander.CommanderID), int64(weekStart), int64(4), int64(monthKey))

	request := &protobuf.CS_26124{Times: proto.Uint32(5)}
	buffer, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := GameRoomExchangeCoin(&buffer, client); err != nil {
		t.Fatalf("exchange coin failed: %v", err)
	}
	resp := &protobuf.SC_26125{}
	decodePacketAt(t, client, 0, 26125, resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected exchange success")
	}
	coin := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(11))
	gold := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(1))
	if coin != 40 {
		t.Fatalf("expected clamped coin to 40, got %d", coin)
	}
	if gold != 7600 {
		t.Fatalf("expected gold cost 2400, got %d", 10000-gold)
	}
	state, err := orm.LoadGameRoomState(client.Commander.CommanderID, now)
	if err != nil {
		t.Fatalf("load game room state: %v", err)
	}
	if state.PayCoinCount != 6 {
		t.Fatalf("expected pay coin count incremented to 6, got %d", state.PayCoinCount)
	}

	projectionClient := &connection.Client{Commander: &orm.Commander{CommanderID: client.Commander.CommanderID}}
	if _, _, err := EventData(&[]byte{}, projectionClient); err != nil {
		t.Fatalf("event data failed: %v", err)
	}
	projection := &protobuf.SC_26120{}
	decodePacketAt(t, projectionClient, 0, 26120, projection)
	if projection.GetPayCoinCount() != 6 {
		t.Fatalf("expected projected pay_coin_count 6, got %d", projection.GetPayCoinCount())
	}
}

func TestGameRoomExchangeCoinValidationFailures(t *testing.T) {
	client := setupGameRoomTestClient(t, 9813, 100, 0, 0)

	zeroTimes, _ := proto.Marshal(&protobuf.CS_26124{Times: proto.Uint32(0)})
	if _, _, err := GameRoomExchangeCoin(&zeroTimes, client); err != nil {
		t.Fatalf("times=0 exchange failed unexpectedly: %v", err)
	}
	zeroResp := &protobuf.SC_26125{}
	offset := decodePacketAt(t, client, 0, 26125, zeroResp)
	if zeroResp.GetResult() == 0 {
		t.Fatalf("expected times=0 to fail")
	}

	oneTime, _ := proto.Marshal(&protobuf.CS_26124{Times: proto.Uint32(1)})
	if _, _, err := GameRoomExchangeCoin(&oneTime, client); err != nil {
		t.Fatalf("insufficient-gold exchange failed unexpectedly: %v", err)
	}
	insufficient := &protobuf.SC_26125{}
	decodePacketAt(t, client, offset, 26125, insufficient)
	if insufficient.GetResult() == 0 {
		t.Fatalf("expected insufficient gold to fail")
	}
}

func TestGameRoomExchangeCoinMapsTxUnderflowToFailureResponse(t *testing.T) {
	client := setupGameRoomTestClient(t, 9817, 0, 0, 0)
	client.Commander.OwnedResourcesMap[1].Amount = 1200

	oneTime, _ := proto.Marshal(&protobuf.CS_26124{Times: proto.Uint32(1)})
	if _, _, err := GameRoomExchangeCoin(&oneTime, client); err != nil {
		t.Fatalf("tx underflow exchange should return failure response, got error: %v", err)
	}
	resp := &protobuf.SC_26125{}
	decodePacketAt(t, client, 0, 26125, resp)
	if resp.GetResult() == 0 {
		t.Fatalf("expected tx underflow to map to insufficient response")
	}
}

func TestGameRoomSuccessSettlementCapsAndScore(t *testing.T) {
	client := setupGameRoomTestClient(t, 9814, 9999, 5, 49990)
	now := time.Now().UTC()
	weekStart := orm.CurrentWeeklyResetUnix(now)
	monthKey := uint32(now.Year()*100 + int(now.Month()))
	execAnswerTestSQLT(t, `
INSERT INTO game_room_states (commander_id, week_start_unix, weekly_claimed, pay_coin_count, first_enter_claimed, month_key, monthly_ticket)
VALUES ($1, $2, false, 0, false, $3, 9995)
`, int64(client.Commander.CommanderID), int64(weekStart), int64(monthKey))

	request := &protobuf.CS_26126{Roomid: proto.Uint32(1), Times: proto.Uint32(2), Score: proto.Uint32(31)}
	buffer, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := GameRoomSuccessSettlement(&buffer, client); err != nil {
		t.Fatalf("settlement failed: %v", err)
	}
	resp := &protobuf.SC_26127{}
	offset := decodePacketAt(t, client, 0, 26127, resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected settlement success")
	}
	if len(resp.GetDropList()) != 1 || resp.GetDropList()[0].GetNumber() != 5 {
		t.Fatalf("expected clamped drop_list reward of 5")
	}
	coin := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(11))
	ticket := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(12))
	if coin != 3 {
		t.Fatalf("expected game coin consumption, got %d", coin)
	}
	if ticket != 49995 {
		t.Fatalf("expected ticket to increase by 5, got %d", ticket)
	}

	lowerScore, _ := proto.Marshal(&protobuf.CS_26126{Roomid: proto.Uint32(1), Times: proto.Uint32(1), Score: proto.Uint32(10)})
	if _, _, err := GameRoomSuccessSettlement(&lowerScore, client); err != nil {
		t.Fatalf("second settlement failed: %v", err)
	}
	second := &protobuf.SC_26127{}
	decodePacketAt(t, client, offset, 26127, second)
	if second.GetResult() != 0 {
		t.Fatalf("expected second settlement success")
	}

	scores, err := orm.ListGameRoomScores(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list scores: %v", err)
	}
	if len(scores) != 1 || scores[0].MaxScore != 31 {
		t.Fatalf("expected max score to remain 31")
	}

	projectionClient := &connection.Client{Commander: &orm.Commander{CommanderID: client.Commander.CommanderID}}
	if _, _, err := EventData(&[]byte{}, projectionClient); err != nil {
		t.Fatalf("event data failed: %v", err)
	}
	projection := &protobuf.SC_26120{}
	decodePacketAt(t, projectionClient, 0, 26120, projection)
	if projection.GetMonthlyTicket() != 10000 {
		t.Fatalf("expected projected monthly ticket cap value 10000")
	}
	if len(projection.GetRooms()) != 1 || projection.GetRooms()[0].GetMaxScore() != 31 {
		t.Fatalf("expected projected room max score to be 31")
	}
}

func TestGameRoomFirstEnterCoinClaimAndProjection(t *testing.T) {
	client := setupGameRoomTestClient(t, 9815, 9999, 37, 0)
	request := &protobuf.CS_26128{Type: proto.Uint32(0)}
	buffer, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := GameRoomFirstEnterCoinClaim(&buffer, client); err != nil {
		t.Fatalf("first enter claim failed: %v", err)
	}
	resp := &protobuf.SC_26129{}
	offset := decodePacketAt(t, client, 0, 26129, resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected first enter success")
	}
	coin := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(11))
	if coin != 40 {
		t.Fatalf("expected cap-clamped first-enter coin grant")
	}

	if _, _, err := GameRoomFirstEnterCoinClaim(&buffer, client); err != nil {
		t.Fatalf("second first enter claim failed: %v", err)
	}
	second := &protobuf.SC_26129{}
	decodePacketAt(t, client, offset, 26129, second)
	if second.GetResult() == 0 {
		t.Fatalf("expected second first-enter claim to fail")
	}

	projectionClient := &connection.Client{Commander: &orm.Commander{CommanderID: client.Commander.CommanderID}}
	if _, _, err := EventData(&[]byte{}, projectionClient); err != nil {
		t.Fatalf("event data failed: %v", err)
	}
	projection := &protobuf.SC_26120{}
	decodePacketAt(t, projectionClient, 0, 26120, projection)
	if projection.GetFirstEnter() != 1 {
		t.Fatalf("expected first_enter to be claimed")
	}
}

func TestGameRoomSuccessSettlementInvalidRoomFails(t *testing.T) {
	client := setupGameRoomTestClient(t, 9816, 9999, 5, 0)
	request := &protobuf.CS_26126{Roomid: proto.Uint32(999), Times: proto.Uint32(1), Score: proto.Uint32(10)}
	buffer, _ := proto.Marshal(request)
	if _, _, err := GameRoomSuccessSettlement(&buffer, client); err != nil {
		t.Fatalf("invalid room settlement failed unexpectedly: %v", err)
	}
	resp := &protobuf.SC_26127{}
	decodePacketAt(t, client, 0, 26127, resp)
	if resp.GetResult() == 0 {
		t.Fatalf("expected invalid room result to be non-zero")
	}
	if len(resp.GetDropList()) != 0 {
		t.Fatalf("expected empty drop list for invalid room")
	}
	remainingCoin := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(11))
	if remainingCoin != 5 {
		t.Fatalf("expected no coin spend on invalid room")
	}
}

func TestGameRoomSuccessSettlementMapsTxUnderflowToFailureResponse(t *testing.T) {
	client := setupGameRoomTestClient(t, 9818, 9999, 0, 0)
	client.Commander.OwnedResourcesMap[11].Amount = 1

	request := &protobuf.CS_26126{Roomid: proto.Uint32(1), Times: proto.Uint32(1), Score: proto.Uint32(10)}
	buffer, _ := proto.Marshal(request)
	if _, _, err := GameRoomSuccessSettlement(&buffer, client); err != nil {
		t.Fatalf("tx underflow settlement should return failure response, got error: %v", err)
	}
	resp := &protobuf.SC_26127{}
	decodePacketAt(t, client, 0, 26127, resp)
	if resp.GetResult() == 0 {
		t.Fatalf("expected tx underflow to map to insufficient response")
	}
	if len(resp.GetDropList()) != 0 {
		t.Fatalf("expected empty drop list on failure")
	}
}
