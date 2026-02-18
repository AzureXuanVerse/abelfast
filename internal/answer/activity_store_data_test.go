package answer

import (
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedActivityStoreTemplate(t *testing.T, actID uint32, start time.Time, stop time.Time) {
	t.Helper()
	payload := fmt.Sprintf(`{"id":%d,"type":1,"config_data":[],"time":["timer",[[%d,%d,%d],[%d,%d,%d]],[[%d,%d,%d],[%d,%d,%d]],1,1]}`,
		actID,
		start.Year(), int(start.Month()), start.Day(), start.Hour(), start.Minute(), start.Second(),
		stop.Year(), int(stop.Month()), stop.Day(), stop.Hour(), stop.Minute(), stop.Second(),
	)
	seedConfigEntry(t, "ShareCfg/activity_template.json", fmt.Sprintf("%d", actID), payload)
}

func TestActivityStoreDataSuccessAndDefaults(t *testing.T) {
	client := setupNewServerShopTest(t)
	start := time.Now().UTC().Add(-time.Hour)
	stop := time.Now().UTC().Add(time.Hour)
	seedActivityStoreTemplate(t, 6001, start, stop)

	request := &protobuf.CS_26160{ActId: proto.Uint32(6001), IntValue: proto.Uint32(77), StrValue: proto.String("abc")}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := ActivityStoreData(&buf, client); err != nil {
		t.Fatalf("ActivityStoreData: %v", err)
	}

	var resp protobuf.SC_26161
	decodePacketAt(t, client, 0, 26161, &resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected success, got %d", resp.GetResult())
	}

	stored, err := orm.GetActivityStoreState(client.Commander.CommanderID, 6001)
	if err != nil {
		t.Fatalf("load activity store state: %v", err)
	}
	if stored.Data1 != 77 || stored.StrData1 != "abc" {
		t.Fatalf("unexpected persisted values: %+v", stored)
	}

	requestDefault := &protobuf.CS_26160{ActId: proto.Uint32(6001)}
	bufDefault, err := proto.Marshal(requestDefault)
	if err != nil {
		t.Fatalf("marshal default request: %v", err)
	}
	if _, _, err := ActivityStoreData(&bufDefault, client); err != nil {
		t.Fatalf("ActivityStoreData default: %v", err)
	}
	client.Buffer.Reset()

	storedDefault, err := orm.GetActivityStoreState(client.Commander.CommanderID, 6001)
	if err != nil {
		t.Fatalf("load default activity store state: %v", err)
	}
	if storedDefault.Data1 != 0 || storedDefault.StrData1 != "" {
		t.Fatalf("expected default zero/empty values, got %+v", storedDefault)
	}
}

func TestActivityStoreDataRejectsMissingOrEndedActivity(t *testing.T) {
	client := setupNewServerShopTest(t)

	missingRequest := &protobuf.CS_26160{ActId: proto.Uint32(9001), IntValue: proto.Uint32(1)}
	missingBuf, _ := proto.Marshal(missingRequest)
	if _, _, err := ActivityStoreData(&missingBuf, client); err != nil {
		t.Fatalf("ActivityStoreData missing: %v", err)
	}
	var missingResp protobuf.SC_26161
	decodePacketAt(t, client, 0, 26161, &missingResp)
	if missingResp.GetResult() == 0 {
		t.Fatalf("expected non-zero for missing activity")
	}

	start := time.Now().UTC().Add(-2 * time.Hour)
	stop := time.Now().UTC().Add(-time.Hour)
	seedActivityStoreTemplate(t, 6002, start, stop)
	endedRequest := &protobuf.CS_26160{ActId: proto.Uint32(6002), IntValue: proto.Uint32(1)}
	endedBuf, _ := proto.Marshal(endedRequest)
	if _, _, err := ActivityStoreData(&endedBuf, client); err != nil {
		t.Fatalf("ActivityStoreData ended: %v", err)
	}
	var endedResp protobuf.SC_26161
	decodePacketAt(t, client, 0, 26161, &endedResp)
	if endedResp.GetResult() == 0 {
		t.Fatalf("expected non-zero for ended activity")
	}
}
