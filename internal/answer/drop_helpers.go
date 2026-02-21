package answer

import (
	"fmt"
	"sort"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func mergeDropList(drops []*protobuf.DROPINFO) []*protobuf.DROPINFO {
	merged := make(map[string]*protobuf.DROPINFO, len(drops))
	for _, drop := range drops {
		key := fmt.Sprintf("%d_%d", drop.GetType(), drop.GetId())
		existing := merged[key]
		if existing == nil {
			merged[key] = &protobuf.DROPINFO{
				Type:   proto.Uint32(drop.GetType()),
				Id:     proto.Uint32(drop.GetId()),
				Number: proto.Uint32(drop.GetNumber()),
			}
			continue
		}
		existing.Number = proto.Uint32(existing.GetNumber() + drop.GetNumber())
	}
	out := make([]*protobuf.DROPINFO, 0, len(merged))
	for _, drop := range merged {
		out = append(out, drop)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GetType() == out[j].GetType() {
			return out[i].GetId() < out[j].GetId()
		}
		return out[i].GetType() < out[j].GetType()
	})
	return out
}
