package educate

import (
	"encoding/json"
	"sort"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	educateLegacyCategory = "ShareCfg/child_site.json"
)

type educateSiteConfig struct {
	ID           uint32    `json:"id"`
	OptionRandom [][][]any `json:"option_random"`
}

func EducateRequestOption(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_27045
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 27046, err
	}

	options, err := loadEducateSiteOptions()
	if err != nil {
		return 0, 27046, err
	}

	response := protobuf.SC_27046{
		Result: proto.Uint32(0),
		Opts:   options,
	}
	return client.SendMessage(27046, &response)
}

func loadEducateSiteOptions() ([]*protobuf.CHILD_SITE_OPTION, error) {
	entries, err := orm.ListConfigEntries(educateLegacyCategory)
	if err != nil {
		return defaultEducateSiteOptions(), nil
	}
	if len(entries) == 0 {
		return defaultEducateSiteOptions(), nil
	}

	options := make([]*protobuf.CHILD_SITE_OPTION, 0, len(entries))
	for _, entry := range entries {
		var cfg educateSiteConfig
		if err := json.Unmarshal(entry.Data, &cfg); err != nil {
			continue
		}
		if cfg.ID == 0 || len(cfg.OptionRandom) == 0 {
			continue
		}

		optionIDs := firstOptionIDsFromBuckets(cfg.OptionRandom)
		if len(optionIDs) == 0 {
			continue
		}

		options = append(options, &protobuf.CHILD_SITE_OPTION{
			SiteId:    proto.Uint32(cfg.ID),
			OptionIds: optionIDs,
		})
	}

	sort.Slice(options, func(i, j int) bool {
		return options[i].GetSiteId() < options[j].GetSiteId()
	})
	if len(options) == 0 {
		return defaultEducateSiteOptions(), nil
	}

	return options, nil
}

func defaultEducateSiteOptions() []*protobuf.CHILD_SITE_OPTION {
	return []*protobuf.CHILD_SITE_OPTION{
		{SiteId: proto.Uint32(131), OptionIds: []uint32{1314, 13142}},
		{SiteId: proto.Uint32(141), OptionIds: []uint32{1414, 14142}},
	}
}

func firstOptionIDsFromBuckets(buckets [][][]any) []uint32 {
	optionIDs := make([]uint32, 0, len(buckets))
	for _, bucket := range buckets {
		if len(bucket) == 0 || len(bucket[0]) == 0 {
			continue
		}
		optionID, ok := parseAnyUint32(bucket[0][0])
		if !ok || optionID == 0 {
			continue
		}
		optionIDs = append(optionIDs, optionID)
	}
	return optionIDs
}

func parseAnyUint32(value any) (uint32, bool) {
	switch number := value.(type) {
	case float64:
		return uint32(number), true
	case int:
		return uint32(number), true
	case uint32:
		return number, true
	default:
		return 0, false
	}
}
