package game

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// maxLoadingPics is AppreciatePicConst.MAX_COUNT: how many pictures the
// loading screen picks from, gallery and manga together.
const maxLoadingPics = 12

// handleChangeCover answers ChangeLivingAreaCoverCommand (the port's
// wallpaper, picked in the Dorm 3D apartment / LivingAreaCover layer): the
// livingarea_cover id the port shows, back in SC_11003.cover.id. 0 is the
// default, any other must be unlocked (a DROP_TYPE_LIVINGAREA_COVER drop,
// SC_11003.cover.covers). A refusal shows ERROR_MESSAGE[result]. Never
// captured: built from the Lua alone.
func (game *Game) handleChangeCover(client *connection.Client, req *protobuf.CS_11030) error {
	ctx := context.Background()
	id := req.GetLivingareaCoverId()
	if _, ok := covers[id]; !ok {
		return client.Send(&protobuf.SC_11031{Result: result(errRefused)})
	}
	owned, err := ownsAttire(ctx, game.DB.Queries, client.CommanderID, dropTypeCover, id)
	if err != nil {
		return err
	}
	if !owned {
		return client.Send(&protobuf.SC_11031{Result: result(errRefused)})
	}
	if err := game.DB.SetCover(ctx, db.SetCoverParams{CommanderID: client.CommanderID, Cover: id}); err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11031{Result: result(nil)})
}

// handleLoadingPics answers UpdateLoadingPicCommand (Attire screen, Loading
// Screen tab; also the gallery and manga viewers' "use as loading screen"):
// whether the loading screen shows the commander's own picks, and the picks,
// gallery_config ids then cartoon ids, back in SC_11003.loading_pic_*
// (LoadingPicProxy). As the client checks: at most MAX_COUNT (12) together
// (loading_pic_max), every id a known picture; the attire panel offers
// every picture whatever is unlocked in the Archives, so neither list is
// checked against the unlocks. Never captured: built from the Lua alone.
func (game *Game) handleLoadingPics(client *connection.Client, req *protobuf.CS_11034) error {
	gallery, manga := req.GetLoadingPicIdList_1(), req.GetLoadingPicIdList_2()
	if req.GetLoadingPicOpenFlag() > 1 || len(gallery)+len(manga) > maxLoadingPics || !loadingPicsKnown(gallery, manga) {
		return client.Send(&protobuf.SC_11035{Result: result(errRefused)})
	}
	err := game.DB.SetLoadingPics(context.Background(), db.SetLoadingPicsParams{
		CommanderID:    client.CommanderID,
		LoadingPicOpen: req.GetLoadingPicOpenFlag() == 1,
		GalleryPics:    gallery,
		MangaPics:      manga,
	})
	if err != nil {
		return err
	}
	return client.Send(&protobuf.SC_11035{Result: result(nil)})
}

// loadingPicsKnown tells whether every id is a gallery_config (gallery) or
// cartoon (manga) row, each once.
func loadingPicsKnown(gallery, manga []uint32) bool {
	seen := map[[2]uint32]bool{}
	for kind, list := range [][]uint32{gallery, manga} {
		for _, id := range list {
			key := [2]uint32{uint32(kind), id}
			if seen[key] {
				return false
			}
			seen[key] = true
			if kind == 0 {
				if _, ok := galleryEntries[id]; !ok {
					return false
				}
			} else if _, ok := cartoonEntries[id]; !ok {
				return false
			}
		}
	}
	return true
}
