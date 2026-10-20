package game

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// The Archives' appreciation pages (view/collection): the gallery pictures
// (gallery_config), the songs of the music player (music_collect_config)
// and the manga chapters (cartoon), kept by AppreciateProxy from SC_11003.
// None of their packets were captured: every reply is built from the Lua
// alone.

// commander_appreciation kinds: what a row records about its item
const (
	appreciationGalleryUnlocked = 1
	appreciationMusicUnlocked   = 2
	appreciationGalleryLiked    = 3
	appreciationMusicLiked      = 4
	appreciationMangaRead       = 5
	appreciationMangaLiked      = 6
)

// like actions: GalleryLikeCommand / MusicLikeCommand isAdd and
// MangaConst.SET_MANGA_LIKE / CANCEL_MANGA_LIKE
const (
	appreciationLike   = 0
	appreciationUnlike = 1
)

// music player album (CS_17513 music_no, AppreciateProxy:
// getMainPlayerAlbumName): none, the liked songs, or else the first
// music_collect_config id of an album
const (
	musicAlbumNone  = 0
	musicAlbumFavor = 999
)

// musicModes are the loop types (MusicPlayLoopTypeChangeCommand): list,
// random, one.
const musicModes = 3

// appreciationEntry is a gallery_config or music_collect_config row:
// AppreciateProxy:isPicNeedUnlockByID / isPicUnlockableByID read its
// unlock_level {commander level, show while locked} and unlock_cost
// ([type, id, count] drops paid by GalleryUnlockCommand).
type appreciationEntry struct {
	UnlockLevel []uint32   `json:"unlock_level"`
	UnlockCost  [][]uint32 `json:"unlock_cost"`
}

// needsUnlock is isPicNeedUnlockByID: an entry is open from the start when
// it asks for level 1 and costs nothing.
func (entry appreciationEntry) needsUnlock() bool {
	return len(entry.UnlockLevel) < 2 || entry.UnlockLevel[0] != 1 || entry.UnlockLevel[1] != 0 || len(entry.UnlockCost) > 0
}

// unlockable is isPicUnlockableByID: the commander reached its level.
func (entry appreciationEntry) unlockable(level uint32) bool {
	return len(entry.UnlockLevel) > 0 && level >= entry.UnlockLevel[0]
}

// cartoonEntry is a cartoon row (MangaConst), only its id matters here.
type cartoonEntry struct {
	Mark uint32 `json:"mark"`
}

var (
	galleryEntries map[uint32]appreciationEntry
	musicEntries   map[uint32]appreciationEntry
	cartoonEntries map[uint32]cartoonEntry
)

func loadAppreciationData() error {
	return gamedata.LoadAll(
		gamedata.Load("gallery_config", &galleryEntries),
		gamedata.Load("music_collect_config", &musicEntries),
		gamedata.Load("cartoon", &cartoonEntries),
	)
}

// appreciationLogin is SC_11003's appreciation and cartoon_read_mark /
// cartoon_collect_mark: Player.Ctor adds the lists to AppreciateProxy in
// order, and initMangaReadIDList / initMangaLikeIDList read the marks as
// bitsets, bit (id-1)%32 of word (id-1)/32.
func appreciationLogin(ctx context.Context, queries *db.Queries, commanderID uint32) (info *protobuf.APPRECIATIONINFO, read, liked []uint32, err error) {
	rows, err := queries.ListAppreciation(ctx, commanderID)
	if err != nil {
		return nil, nil, nil, err
	}
	player, err := queries.GetMusicPlayer(ctx, commanderID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, err
	}
	info = &protobuf.APPRECIATIONINFO{MusicNo: proto.Uint32(player.MusicNo), MusicMode: proto.Uint32(player.MusicMode)}
	for _, row := range rows {
		switch row.Kind {
		case appreciationGalleryUnlocked:
			info.Gallerys = append(info.Gallerys, row.ItemID)
		case appreciationMusicUnlocked:
			info.Musics = append(info.Musics, row.ItemID)
		case appreciationGalleryLiked:
			info.FavorGallerys = append(info.FavorGallerys, row.ItemID)
		case appreciationMusicLiked:
			info.FavorMusics = append(info.FavorMusics, row.ItemID)
		case appreciationMangaRead:
			read = setMangaMark(read, row.ItemID)
		case appreciationMangaLiked:
			liked = setMangaMark(liked, row.ItemID)
		}
	}
	return info, read, liked, nil
}

func setMangaMark(marks []uint32, id uint32) []uint32 {
	if id == 0 {
		return marks
	}
	word := int((id - 1) / 32)
	for len(marks) <= word {
		marks = append(marks, 0)
	}
	marks[word] |= 1 << ((id - 1) % 32)
	return marks
}

func (tx *Tx) hasAppreciation(kind, id uint32) (bool, error) {
	count, err := tx.CountAppreciation(tx.ctx, db.CountAppreciationParams{CommanderID: tx.commanderID, Kind: kind, ItemID: id})
	return count > 0, err
}

// unlockAppreciation is GalleryUnlockCommand / MusicUnlockCommand: an entry
// still locked, at the commander's level, paid with its unlock_cost.
func (tx *Tx) unlockAppreciation(entries map[uint32]appreciationEntry, kind, id uint32) error {
	entry, ok := entries[id]
	if !ok {
		return errRefused
	}
	commander, err := tx.GetCommander(tx.ctx, tx.commanderID)
	if err != nil {
		return err
	}
	if !entry.unlockable(commander.Level) {
		return errRefused
	}
	added, err := tx.AddAppreciation(tx.ctx, db.AddAppreciationParams{CommanderID: tx.commanderID, Kind: kind, ItemID: id})
	if err != nil {
		return err
	}
	if added == 0 {
		return errRefused
	}
	return tx.payDrops(entry.UnlockCost)
}

// likeAppreciation adds id to or removes it from a like list. Removing an
// id that isn't liked goes through, as the proxy only prints it.
func (tx *Tx) likeAppreciation(kind, id, action uint32) error {
	switch action {
	case appreciationLike:
		_, err := tx.AddAppreciation(tx.ctx, db.AddAppreciationParams{CommanderID: tx.commanderID, Kind: kind, ItemID: id})
		return err
	case appreciationUnlike:
		return tx.RemoveAppreciation(tx.ctx, db.RemoveAppreciationParams{CommanderID: tx.commanderID, Kind: kind, ItemID: id})
	}
	return errRefused
}

// likeUnlocked likes a gallery picture or a song the commander can see:
// one open from the start or unlocked (isPicNeedUnlockByID,
// isPicUnlockedByID).
func (tx *Tx) likeUnlocked(entries map[uint32]appreciationEntry, unlockedKind, likedKind, id, action uint32) error {
	entry, ok := entries[id]
	if !ok {
		return errRefused
	}
	if action == appreciationLike && entry.needsUnlock() {
		unlocked, err := tx.hasAppreciation(unlockedKind, id)
		if err != nil {
			return err
		}
		if !unlocked {
			return errRefused
		}
	}
	return tx.likeAppreciation(likedKind, id, action)
}

// handleGalleryUnlock answers GalleryUnlockCommand (CS_17501): unlocks a
// gallery_config picture and pays its unlock_cost, which the client then
// takes off its own resources and bag. No page sends it in this client
// version (every picture is open from the start).
func (game *Game) handleGalleryUnlock(client *connection.Client, req *protobuf.CS_17501) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		return tx.unlockAppreciation(galleryEntries, appreciationGalleryUnlocked, req.GetId())
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17502{Result: result(err)})
}

// handleMusicUnlock answers MusicUnlockCommand (CS_17503), the
// music_collect_config counterpart of handleGalleryUnlock.
func (game *Game) handleMusicUnlock(client *connection.Client, req *protobuf.CS_17503) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		return tx.unlockAppreciation(musicEntries, appreciationMusicUnlocked, req.GetId())
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17504{Result: result(err)})
}

// handleGalleryLike answers GalleryLikeCommand (CS_17505, the heart of a
// picture in Archives > Memories gallery or the picture viewer): action 0
// adds the picture to favor_gallerys, 1 removes it.
func (game *Game) handleGalleryLike(client *connection.Client, req *protobuf.CS_17505) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		return tx.likeUnlocked(galleryEntries, appreciationGalleryUnlocked, appreciationGalleryLiked, req.GetId(), req.GetAction())
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17506{Result: result(err)})
}

// handleMusicLike answers MusicLikeCommand (CS_17507, the heart of a song
// in the music collection or the main scene's player): action 0 adds the
// song to favor_musics, the "favor" album, 1 removes it.
func (game *Game) handleMusicLike(client *connection.Client, req *protobuf.CS_17507) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		return tx.likeUnlocked(musicEntries, appreciationMusicUnlocked, appreciationMusicLiked, req.GetId(), req.GetAction())
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17508{Result: result(err)})
}

// handleMangaRead answers MangaReadCommand (CS_17509, sent by
// MangaFullScreenLayer the first time a cartoon chapter is opened): marks
// it read in cartoon_read_mark.
func (game *Game) handleMangaRead(client *connection.Client, req *protobuf.CS_17509) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		if _, ok := cartoonEntries[req.GetId()]; !ok {
			return errRefused
		}
		_, err := tx.AddAppreciation(tx.ctx, db.AddAppreciationParams{CommanderID: tx.commanderID, Kind: appreciationMangaRead, ItemID: req.GetId()})
		return err
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17510{Result: result(err)})
}

// handleMangaLike answers MangaLikeCommand (CS_17511, the heart of a
// chapter in the manga viewer): action 0 (SET_MANGA_LIKE) sets its bit in
// cartoon_collect_mark, 1 (CANCEL_MANGA_LIKE) clears it.
func (game *Game) handleMangaLike(client *connection.Client, req *protobuf.CS_17511) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		if _, ok := cartoonEntries[req.GetId()]; !ok {
			return errRefused
		}
		return tx.likeAppreciation(appreciationMangaLiked, req.GetId(), req.GetAction())
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17512{Result: result(err)})
}

// handleMusicPlayer answers MusicMainPlayAlbumChangeCommand and
// MusicPlayLoopTypeChangeCommand (CS_17513, the album's home button and the
// loop button of the music collection or the main scene's player): both
// send the whole player, music_no the album (0 none, 999 liked songs, else
// its first song) and music_mode the loop type, sent back in
// SC_11003.appreciation.
func (game *Game) handleMusicPlayer(client *connection.Client, req *protobuf.CS_17513) error {
	err := game.tx(context.Background(), client, func(tx *Tx) error {
		no := req.GetMusicNo()
		if _, ok := musicEntries[no]; !ok && no != musicAlbumNone && no != musicAlbumFavor {
			return errRefused
		}
		if req.GetMusicMode() >= musicModes {
			return errRefused
		}
		return tx.SetMusicPlayer(tx.ctx, db.SetMusicPlayerParams{CommanderID: tx.commanderID, MusicNo: no, MusicMode: req.GetMusicMode()})
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17514{Result: result(err)})
}
