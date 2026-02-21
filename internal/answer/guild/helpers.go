package guild

import (
	"time"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func nowUnix() uint32 {
	return uint32(time.Now().Unix())
}

func containsUint32(list []uint32, value uint32) bool {
	for _, entry := range list {
		if entry == value {
			return true
		}
	}
	return false
}

func buildDisplayInfo(profile orm.CommanderSocialProfile) *protobuf.DISPLAYINFO {
	return &protobuf.DISPLAYINFO{
		Icon:          proto.Uint32(profile.DisplayIconID),
		Skin:          proto.Uint32(profile.DisplaySkinID),
		IconFrame:     proto.Uint32(profile.SelectedIconFrameID),
		ChatFrame:     proto.Uint32(profile.SelectedChatFrameID),
		IconTheme:     proto.Uint32(profile.DisplayIconThemeID),
		MarryFlag:     proto.Uint32(0),
		TransformFlag: proto.Uint32(0),
	}
}
