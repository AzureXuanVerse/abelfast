package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestSendSpecialWeaponSyncSerializesOwnedWeapons(t *testing.T) {
	client := &connection.Client{
		Commander: &orm.Commander{
			OwnedSpWeapons: []orm.OwnedSpWeapon{
				{ID: 11, TemplateID: 101, Attr1: 1, Attr2: 2, AttrTemp1: 3, AttrTemp2: 4, Effect: 5, Pt: 6},
				{ID: 12, TemplateID: 202, Attr1: 7, Attr2: 8, AttrTemp1: 9, AttrTemp2: 10, Effect: 11, Pt: 12},
			},
		},
	}

	if _, _, err := SendSpecialWeaponSync(client); err != nil {
		t.Fatalf("send special weapon sync failed: %v", err)
	}

	var response protobuf.SC_14200
	decodePacketAt(t, client, 0, 14200, &response)

	if len(response.GetSpweaponList()) != 2 {
		t.Fatalf("expected 2 special weapons, got %d", len(response.GetSpweaponList()))
	}

	first := response.GetSpweaponList()[0]
	if first.GetId() != 11 || first.GetTemplateId() != 101 {
		t.Fatalf("unexpected first spweapon identity: id=%d template=%d", first.GetId(), first.GetTemplateId())
	}
	if first.GetAttr_1() != 1 || first.GetAttr_2() != 2 || first.GetAttrTemp_1() != 3 || first.GetAttrTemp_2() != 4 || first.GetEffect() != 5 || first.GetPt() != 6 {
		t.Fatalf("unexpected first spweapon attributes")
	}
}

func TestSendSpecialWeaponSyncUsesEmptyListWhenNoWeapons(t *testing.T) {
	client := &connection.Client{Commander: &orm.Commander{}}

	if _, _, err := SendSpecialWeaponSync(client); err != nil {
		t.Fatalf("send special weapon sync failed: %v", err)
	}

	var response protobuf.SC_14200
	decodePacketAt(t, client, 0, 14200, &response)

	if len(response.GetSpweaponList()) != 0 {
		t.Fatalf("expected empty special weapon list, got %d", len(response.GetSpweaponList()))
	}
}
