package game

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// item_data_statistics usages (ItemUsage, model/const/itemusage.lua) that
// give ship skins
const (
	usageRandomSkin   = "usage_drop_random_skin"
	usageSkinSelect   = "usage_drop_appointed_skinexchange" // ItemUsage.SKIN_SELECT
	usageSkinDiscount = "usage_skin_discount"               // ItemUsage.SKIN_SHOP_DISCOUNT
	usageSkinExp      = "usage_skin_exp"                    // a skin trial voucher
)

// skinUse is the usage_arg of a skin item.
type skinUse struct {
	// usage_drop_random_skin [drop, gems, skins]: a random skin of skins
	// the commander doesn't have, gems when they have them all (the item
	// text). drop is a server-side table, unknown. extras: the display_icon
	// rewards that come with it, without the placeholder of the skin
	// (86212...: a virtual item listed as an item).
	// usage_drop_appointed_skinexchange [activity, skins while it runs,
	// skins, _, item]: the pick of the skins (Item:GetValidSkinList,
	// model/vo/item.lua), item when the commander has them all for good.
	activity uint32
	skins    []uint32
	extras   [][]uint32
	gems     uint32
	limited  []uint32
	item     uint32
	// usage_skin_discount [shops, discount]: the skin_shop rows it buys
	// ({0} any), discount off their price (Item:CanUseForShop).
	// usage_skin_exp [shop]: the skin_shop_timelimit row it is a trial of.
	shops    []uint32
	discount uint32
}

// readSkinUse reads the usage_arg of a skin item, items being
// item_data_statistics.
func readSkinUse(row itemConfig, icons [][]uint32, items map[uint32]itemConfig) skinUse {
	var use skinUse
	var args []json.RawMessage
	if json.Unmarshal(row.UsageArg, &args) != nil {
		return use
	}
	arg := func(i int) []uint32 {
		if i >= len(args) {
			return nil
		}
		return gamedata.IDs(args[i])
	}
	first := func(i int) uint32 {
		if ids := arg(i); len(ids) > 0 {
			return ids[0]
		}
		return 0
	}
	switch row.Usage {
	case usageRandomSkin:
		use.gems, use.skins = first(1), arg(2)
		for _, icon := range icons {
			if len(icon) < 3 {
				continue
			}
			if _, item := items[icon[1]]; icon[0] != dropTypeItem || item {
				use.extras = append(use.extras, icon)
			}
		}
	case usageSkinSelect:
		use.activity, use.limited, use.skins, use.item = first(0), arg(1), arg(2), first(4)
	case usageSkinDiscount:
		// read as is: gamedata.IDs would drop the 0 of {0}, any skin
		if len(args) > 0 {
			json.Unmarshal(args[0], &use.shops)
		}
		use.discount = first(1)
	case usageSkinExp:
		use.shops = arg(0)
	}
	return use
}

// skinItemDrops is what count of a skin item gives (UseItemCommand adds it
// as drop_list, PlayerConst.addTranDrop), with arg what the client sent,
// paying what the item leaves of a skin's price. useItem spends the items
// and grants the drops.
func (tx *Tx) skinItemDrops(count uint32, arg []uint32, use itemUse) ([]*protobuf.DROPINFO, error) {
	now := time.Now()
	owned, err := ownedSkins(tx.ctx, tx.Queries, tx.commanderID, now)
	if err != nil {
		return nil, err
	}
	forGood := func(skin uint32) bool {
		end, ok := owned[skin]
		return ok && end == 0
	}
	skin := use.skin
	switch use.usage {
	case usageRandomSkin:
		// each box draws, with an equal chance, one of the skins the
		// commander doesn't have for good nor drew from the boxes before
		// (the "Mystery" virtual item's text); skins the client couldn't
		// show (storableDrop) aren't in the draw
		pool := slices.DeleteFunc(slices.Clone(skin.skins), func(id uint32) bool {
			return forGood(id) || !storableDrop(dropTypeSkin, id)
		})
		var drops []*protobuf.DROPINFO
		for range count {
			if len(pool) == 0 && skin.gems == 0 {
				return nil, errRefused
			}
			if len(pool) == 0 {
				drops = mergeDrop(drops, dropTypeResource, ResourceFreeGem, skin.gems)
				continue
			}
			i := rand.IntN(len(pool))
			drops = mergeDrop(drops, dropTypeSkin, pool[i], 1)
			pool = slices.Delete(pool, i, i+1)
		}
		for _, extra := range skin.extras {
			drops = mergeDrop(drops, extra[0], extra[1], extra[2]*count)
		}
		return drops, nil
	case usageSkinSelect:
		// StoreHouseScene sends one at a time: the picked skin, or 0 for
		// the item when every skin is the commander's for good
		// (Item:IsAllSkinOwner)
		valid := skin.skins
		if _, open := openActivity(skin.activity, now); open {
			valid = append(slices.Clone(skin.limited), skin.skins...)
		}
		if count != 1 || len(arg) != 1 {
			return nil, errRefused
		}
		if arg[0] == 0 {
			if skin.item == 0 || len(valid) == 0 || slices.ContainsFunc(valid, func(id uint32) bool { return !forGood(id) }) {
				return nil, errRefused
			}
			return []*protobuf.DROPINFO{dropInfo(dropTypeItem, skin.item, 1)}, nil
		}
		if !slices.Contains(valid, arg[0]) || forGood(arg[0]) {
			return nil, errRefused
		}
		return []*protobuf.DROPINFO{dropInfo(dropTypeSkin, arg[0], 1)}, nil
	case usageSkinDiscount:
		// arg is the skin_shop row bought with the coupon; the client takes
		// the price less the discount off its resources itself and counts
		// no purchase (Item:GetConsumeForSkinShopDiscount)
		if count != 1 || len(arg) != 1 || !(slices.Equal(skin.shops, []uint32{0}) || slices.Contains(skin.shops, arg[0])) {
			return nil, errRefused
		}
		goods, ok := shopTemplate[arg[0]]
		if !ok || goods.Genre != genreSkinShop || !goods.Time.Open(now) {
			return nil, errRefused
		}
		if err := tx.canBuySkin(goods, 1, now); err != nil {
			return nil, err
		}
		price, ok := goods.price(1, 100, now)
		if !ok {
			return nil, errRefused
		}
		if err := tx.SpendShopResource(goods.ResourceType, price-min(price, skin.discount)); err != nil {
			return nil, err
		}
		return []*protobuf.DROPINFO{dropInfo(dropTypeSkin, goods.target, 1)}, nil
	}
	return nil, errRefused
}

// useSkinTrial spends a skin trial voucher on the skin_shop_timelimit row arg
// names (NewSkinShopMainView:OnExperience4Item, one at a time): the client
// ignores drop_list and gives the skin itself until now + the row's
// time_second, over what was left of it, unless it is the commander's for
// good.
func (tx *Tx) useSkinTrial(id, count uint32, arg []uint32, use itemUse) error {
	now := time.Now()
	if count != 1 || len(arg) != 1 || !slices.Contains(use.skin.shops, arg[0]) {
		return errRefused
	}
	goods, ok := shopTemplate[arg[0]]
	if _, skin := shipSkins[goods.target]; !ok || !skin || goods.Type != shopTypeSkin || goods.Genre != genreSkinShopTimeLimit || goods.TimeSecond <= 0 {
		return errRefused
	}
	owned, err := ownedSkins(tx.ctx, tx.Queries, tx.commanderID, now)
	if err != nil {
		return err
	}
	if end, ok := owned[goods.target]; ok && end == 0 {
		return errRefused
	}
	if err := tx.SpendItem(id, count); err != nil {
		return err
	}
	return tx.SetSkin(tx.ctx, db.SetSkinParams{CommanderID: tx.commanderID, SkinID: goods.target, EndTime: now.Unix() + goods.TimeSecond})
}
