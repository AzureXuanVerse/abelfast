package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func ColoringFetch(buffer *[]byte, client *connection.Client) (int, int, error) {
	payload := &protobuf.CS_26008{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 26001, err
	}
	if err := ensureCommanderLoaded(client, "Coloring/Fetch"); err != nil {
		return 0, 26001, err
	}

	pages, err := loadColoringActivityPages(payload.GetActId())
	if err != nil {
		return 0, 26001, err
	}

	state, err := getOrCreateColoringState(client.Commander.CommanderID, payload.GetActId())
	if err != nil {
		return 0, 26001, err
	}

	colorItemIDs := make(map[uint32]struct{})
	for i := range pages {
		template, loadErr := loadColoringTemplate(pages[i].PageID)
		if loadErr != nil {
			return 0, 26001, loadErr
		}
		for _, itemID := range template.ColorIDList {
			if itemID == 0 {
				continue
			}
			colorItemIDs[itemID] = struct{}{}
		}
	}

	counts := make(map[uint32]uint32, len(colorItemIDs))
	for itemID := range colorItemIDs {
		counts[itemID] = client.Commander.GetItemCount(itemID)
	}

	currentPageID := coloringCurrentPageID(state, pages)
	response := &protobuf.SC_26001{
		Id:        proto.Uint32(currentPageID),
		CellList:  coloringBuildCellListForPage(state, currentPageID),
		ColorList: coloringBuildColorList(counts),
		AwardList: coloringBuildAwardList(state),
		StartTime: proto.Uint32(state.StartTime),
	}
	return connection.SendProtoMessage(26001, client, response)
}
