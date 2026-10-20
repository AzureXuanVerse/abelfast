package game

import (
	"cmp"
	"math/rand/v2"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// An auction lot: the warehouse of auction_collection items the bidders bid
// on (or an ordered box), laid on a grid auctionGridColumns wide, lines
// high (at least auctionGridLines). An item's pos list (PTGC_BID_KING_ITEM)
// is every cell it covers, column by column, its top left cell first:
// AuctionGameStoreItemData takes pos[1] as its position and the extent of
// the list as its contour, {width, height} as auction_collection.contour
// (AuctionGameConst.GUIDE_ITEM_LIST lays the tutorial's lot out so).

// auction_event groups, AuctionGameConst.EVENT_TYPE_GROUP
const (
	auctionEventPersonal = 1
	auctionEventPublic   = 2
)

// auctionPersonalEvents is how many personal events a player picks from
// each round (AuctionGameMainEventLayer's uiEventItemTf1..3, the
// tutorial's three).
const auctionPersonalEvents = 3

// AuctionGameConst.EVENT_TYPE
const (
	auctionEventContourBySize   = 1
	auctionEventPosition        = 2
	auctionEventRarityCount     = 3
	auctionEventRandomRarity    = 4
	auctionEventReveal          = 5
	auctionEventRarityPrice     = 6
	auctionEventMaxCellContour  = 7
	auctionEventRarityContour   = 8
	auctionEventMaxPrice        = 9
	auctionEventMaxCellPrice    = 10
	auctionEventMaxRarity       = 11
	auctionEventTotalCells      = 12
	auctionEventRarityPrices    = 13
	auctionEventAveragePrice    = 14
	auctionEventAllContours     = 16
	auctionEventRarityPositions = 17
	auctionEventContourAverage  = 18
	auctionEventRarityCells     = 19
)

// auctionItem is a lot's item: uid (1-based in the lot), auction_collection
// id and its top left cell (1-based).
type auctionItem struct {
	UID uint32 `json:"uid"`
	ID  uint32 `json:"id"`
	X   uint32 `json:"x"`
	Y   uint32 `json:"y"`
}

type auctionLot struct {
	Items []auctionItem
	Lines uint32
}

func (item auctionItem) config() auctionCollectionConfig { return auction.collection[item.ID] }

// cells is the item's pos list, its top left cell first.
func (item auctionItem) cells() []*protobuf.PTGC_BID_KING_ITEM_POS {
	contour := item.config().Contour
	var cells []*protobuf.PTGC_BID_KING_ITEM_POS
	for x := item.X; x < item.X+contour[0]; x++ {
		for y := item.Y; y < item.Y+contour[1]; y++ {
			cells = append(cells, &protobuf.PTGC_BID_KING_ITEM_POS{X: proto.Uint32(x), Y: proto.Uint32(y)})
		}
	}
	return cells
}

// full is everything about the item (a revealed item, the settlement).
func (item auctionItem) full() *protobuf.PTGC_BID_KING_ITEM {
	config := item.config()
	return &protobuf.PTGC_BID_KING_ITEM{
		Uid:    proto.Uint32(item.UID),
		Id:     proto.Uint32(item.ID),
		Rarity: proto.Uint32(config.Rarity),
		Pos:    item.cells(),
		Price:  proto.Uint32(config.Value),
	}
}

// outline is the item's place and contour, nothing else.
func (item auctionItem) outline() *protobuf.PTGC_BID_KING_ITEM {
	return &protobuf.PTGC_BID_KING_ITEM{Uid: proto.Uint32(item.UID), Pos: item.cells()}
}

// spot is the item's position only (its top left cell).
func (item auctionItem) spot(rarity bool) *protobuf.PTGC_BID_KING_ITEM {
	shown := &protobuf.PTGC_BID_KING_ITEM{Uid: proto.Uint32(item.UID), Pos: item.cells()[:1]}
	if rarity {
		shown.Rarity = proto.Uint32(item.config().Rarity)
	}
	return shown
}

// value is the lot's total value.
func (lot auctionLot) value() uint32 {
	total := uint32(0)
	for _, item := range lot.Items {
		total += item.config().Value
	}
	return total
}

// weighted picks an index of weights at random by weight.
func weighted(weights []uint32) int {
	total := uint32(0)
	for _, weight := range weights {
		total += weight
	}
	if total == 0 {
		return rand.IntN(len(weights))
	}
	n := rand.Uint32N(total)
	for i, weight := range weights {
		if n < weight {
			return i
		}
		n -= weight
	}
	return len(weights) - 1
}

// randomBetween is a uniform draw in [lo, hi].
func randomBetween(bounds [2]uint32) uint32 {
	if bounds[1] <= bounds[0] {
		return bounds[0]
	}
	return bounds[0] + rand.Uint32N(bounds[1]-bounds[0]+1)
}

// newAuctionLot draws a session's lot.
//
// GUESS (no table or Lua says how): a total value is drawn from one of the
// value_range ranges (by weight), the lot gets its rarity_limit items, then
// items of rarity_probability rarities until it is worth that much, never
// more than the range's top nor over auction_maximum grid cells. Items are
// laid out at random positions on the grid, the bigger first, adding grid
// lines only when they don't fit.
func newAuctionLot(session auctionSessionConfig) auctionLot {
	ids := drawAuctionItems(session)
	slices.SortStableFunc(ids, func(a, b uint32) int {
		return cmp.Compare(auction.collection[b].area(), auction.collection[a].area())
	})
	for lines := uint32(auctionGridLines); ; lines++ {
		if lot, ok := placeAuctionItems(ids, lines); ok {
			return lot
		}
	}
}

func drawAuctionItems(session auctionSessionConfig) []uint32 {
	weights := make([]uint32, len(session.values))
	for i, entry := range session.values {
		weights[i] = entry.weight
	}
	bucket := session.values[weighted(weights)]
	target := randomBetween([2]uint32{bucket.lo, bucket.hi})
	maxCells := auctionSetting("maximum")

	var ids []uint32
	total, cells := uint32(0), uint32(0)
	add := func(rarity uint32, capped bool) {
		pool := auction.byRarity[rarity]
		if len(pool) == 0 {
			return
		}
		item := auction.collection[pool[rand.IntN(len(pool))]]
		if maxCells > 0 && cells+item.area() > maxCells && len(ids) > 0 {
			return
		}
		if capped && total+item.Value > bucket.hi {
			return
		}
		ids = append(ids, item.ID)
		total += item.Value
		cells += item.area()
	}
	for _, limit := range session.RarityLimit {
		for range limit[1] {
			add(limit[0], false)
		}
	}
	rarities := make([]uint32, len(session.RarityProbability))
	rarityWeights := make([]uint32, len(session.RarityProbability))
	for i, entry := range session.RarityProbability {
		rarities[i], rarityWeights[i] = entry[0], entry[1]
	}
	for tries := 0; total < target && tries < 500 && len(rarities) > 0; tries++ {
		add(rarities[weighted(rarityWeights)], true)
	}
	if len(ids) == 0 {
		for _, pool := range auction.byRarity {
			ids = append(ids, pool[0])
			break
		}
	}
	return ids
}

// placeAuctionItems lays ids out on a grid of lines lines, false when they
// don't fit.
func placeAuctionItems(ids []uint32, lines uint32) (auctionLot, bool) {
	taken := make([]bool, auctionGridColumns*lines)
	free := func(x, y, w, h uint32) bool {
		for i := x; i < x+w; i++ {
			for j := y; j < y+h; j++ {
				if taken[(j-1)*auctionGridColumns+i-1] {
					return false
				}
			}
		}
		return true
	}
	lot := auctionLot{Lines: lines}
	for n, id := range ids {
		contour := auction.collection[id].Contour
		w, h := contour[0], contour[1]
		if w > auctionGridColumns || h > lines {
			return auctionLot{}, false
		}
		var spots [][2]uint32
		for y := uint32(1); y+h-1 <= lines; y++ {
			for x := uint32(1); x+w-1 <= auctionGridColumns; x++ {
				if free(x, y, w, h) {
					spots = append(spots, [2]uint32{x, y})
				}
			}
		}
		if len(spots) == 0 {
			return auctionLot{}, false
		}
		spot := spots[rand.IntN(len(spots))]
		for i := spot[0]; i < spot[0]+w; i++ {
			for j := spot[1]; j < spot[1]+h; j++ {
				taken[(j-1)*auctionGridColumns+i-1] = true
			}
		}
		lot.Items = append(lot.Items, auctionItem{UID: uint32(n + 1), ID: id, X: spot[0], Y: spot[1]})
	}
	return lot, true
}

// pickItems is up to n of the items at random.
func pickItems(items []auctionItem, n uint32) []auctionItem {
	picked := slices.Clone(items)
	rand.Shuffle(len(picked), func(i, j int) { picked[i], picked[j] = picked[j], picked[i] })
	return picked[:min(int(n), len(picked))]
}

// effect is what event shows of the lot (PTGC_BID_KING_EVENT_EFFECT, read
// by AuctionGameTools.RefreshItemDataByEvent): the items it reveals in
// item_list, or the number it tells in int_value
// (AuctionGameMainRightInfoItem shows it after the event's describe; a
// rarity for MAX_RARITY, a cell count times 100 for
// RARITY_ITEMS_CELL_COUNT).
//
// GUESS: what each event type reveals is read from its describe and from
// the fields RefreshItemDataByEvent applies: the tutorial's effects
// (AuctionGameTools.GuideSelectedEvent) send the whole pos list for
// contours and the top left cell only for positions. The "random" items
// are any of the lot's; averages and per-cell values are rounded down.
func (lot auctionLot) effect(eventID uint32) *protobuf.PTGC_BID_KING_EVENT_EFFECT {
	effect := &protobuf.PTGC_BID_KING_EVENT_EFFECT{EventId: proto.Uint32(eventID)}
	event, ok := auction.events[eventID]
	if !ok {
		return effect
	}
	arg := func(i int) uint32 {
		if i < len(event.ConfigData) {
			return event.ConfigData[i]
		}
		return 0
	}
	ofRarity := func(rarity uint32) []auctionItem {
		var items []auctionItem
		for _, item := range lot.Items {
			if item.config().Rarity == rarity {
				items = append(items, item)
			}
		}
		return items
	}
	show := func(items []auctionItem, as func(auctionItem) *protobuf.PTGC_BID_KING_ITEM) {
		for _, item := range items {
			effect.ItemList = append(effect.ItemList, as(item))
		}
	}
	count := func(value uint32) { effect.IntValue = proto.Uint32(value) }
	average := func(total uint32, n int) uint32 {
		if n == 0 {
			return 0
		}
		return total / uint32(n)
	}
	switch event.Type {
	case auctionEventContourBySize:
		var items []auctionItem
		for _, item := range lot.Items {
			if contour := item.config().Contour; contour[0] == arg(0) && contour[1] == arg(1) {
				items = append(items, item)
			}
		}
		show(items, auctionItem.outline)
	case auctionEventPosition:
		show(pickItems(lot.Items, arg(0)), func(item auctionItem) *protobuf.PTGC_BID_KING_ITEM { return item.spot(false) })
	case auctionEventRarityCount:
		count(uint32(len(ofRarity(arg(0)))))
	case auctionEventRandomRarity:
		show(pickItems(lot.Items, arg(0)), func(item auctionItem) *protobuf.PTGC_BID_KING_ITEM { return item.spot(true) })
	case auctionEventReveal:
		show(pickItems(lot.Items, arg(0)), auctionItem.full)
	case auctionEventRarityPrice, auctionEventRarityPrices:
		total := uint32(0)
		for _, item := range ofRarity(arg(0)) {
			total += item.config().Value
		}
		count(total)
	case auctionEventMaxCellContour:
		var biggest []auctionItem
		for _, item := range lot.Items {
			switch {
			case len(biggest) == 0 || item.config().area() > biggest[0].config().area():
				biggest = []auctionItem{item}
			case item.config().area() == biggest[0].config().area():
				biggest = append(biggest, item)
			}
		}
		show(pickItems(biggest, 1), auctionItem.outline)
	case auctionEventRarityContour:
		show(ofRarity(arg(0)), func(item auctionItem) *protobuf.PTGC_BID_KING_ITEM {
			shown := item.outline()
			shown.Rarity = proto.Uint32(item.config().Rarity)
			return shown
		})
	case auctionEventMaxPrice:
		best := uint32(0)
		for _, item := range lot.Items {
			best = max(best, item.config().Value)
		}
		count(best)
	case auctionEventMaxCellPrice:
		best := uint32(0)
		for _, item := range lot.Items {
			best = max(best, item.config().Value/item.config().area())
		}
		count(best)
	case auctionEventMaxRarity:
		best := uint32(0)
		for _, item := range lot.Items {
			best = max(best, item.config().Rarity)
		}
		count(best)
	case auctionEventTotalCells:
		cells := uint32(0)
		for _, item := range lot.Items {
			cells += item.config().area()
		}
		count(cells)
	case auctionEventAveragePrice:
		count(average(lot.value(), len(lot.Items)))
	case auctionEventAllContours:
		show(lot.Items, auctionItem.outline)
	case auctionEventRarityPositions:
		show(ofRarity(arg(0)), func(item auctionItem) *protobuf.PTGC_BID_KING_ITEM { return item.spot(true) })
	case auctionEventContourAverage:
		total, n := uint32(0), 0
		for _, item := range lot.Items {
			if contour := item.config().Contour; contour[0] == arg(0) && contour[1] == arg(1) {
				total += item.config().Value
				n++
			}
		}
		count(average(total, n))
	case auctionEventRarityCells:
		items := ofRarity(arg(0))
		cells := uint32(0)
		for _, item := range items {
			cells += item.config().area()
		}
		count(average(cells*100, len(items)))
	}
	return effect
}
