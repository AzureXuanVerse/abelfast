package chapter

import "github.com/ggmolly/belfast/internal/protobuf"

type ChapterTemplate = chapterTemplate
type ChapterPos = chapterPos

const (
	ChapterOpMove   = chapterOpMove
	ChapterOpAmbush = chapterOpAmbush

	ChapterAttachBoss         = chapterAttachBoss
	ChapterAttachElite        = chapterAttachElite
	ChapterAttachAmbush       = chapterAttachAmbush
	ChapterAttachEnemy        = chapterAttachEnemy
	ChapterAttachTorpedoEnemy = chapterAttachTorpedoEnemy
	ChapterAttachChampion     = chapterAttachChampion
	ChapterAttachBombEnemy    = chapterAttachBombEnemy

	ChapterCellActive   = chapterCellActive
	ChapterCellDisabled = chapterCellDisabled
	ChapterCellAmbush   = chapterCellAmbush
)

func LoadChapterTemplate(chapterID uint32, loopFlag uint32) (*ChapterTemplate, error) {
	return loadChapterTemplate(chapterID, loopFlag)
}

func FindChapterCellAt(current *protobuf.CURRENTCHAPTERINFO, pos ChapterPos) (int, *protobuf.CHAPTERCELLINFO_P13) {
	return findChapterCellAt(current, chapterPos(pos))
}

func ParseEliteFleetFromState(state []byte) ([]*protobuf.FLEET_INFO, error) {
	return parseEliteFleetFromState(state)
}

func SetEliteFleetInState(state []byte, fleets []*protobuf.FLEET_INFO) ([]byte, error) {
	return setEliteFleetInState(state, fleets)
}
