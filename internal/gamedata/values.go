package gamedata

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Zone is the EN servers' clock, the one config times are written in.
var Zone = time.FixedZone("UTC-7", -7*60*60)

// IDs reads an id field the tables write several ways: a number, a list
// (nested lists flattened), or either as a Lua string. 0 is no id.
func IDs(raw json.RawMessage) []uint32 {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		text = strings.TrimSpace(text)
		if strings.HasPrefix(text, "[") {
			return IDs(json.RawMessage(text))
		}
		id, err := strconv.ParseUint(text, 10, 32)
		if err != nil || id == 0 {
			return nil
		}
		return []uint32{uint32(id)}
	}
	var number uint32
	if json.Unmarshal(raw, &number) == nil {
		if number == 0 {
			return nil
		}
		return []uint32{number}
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var ids []uint32
	for _, item := range list {
		ids = append(ids, IDs(item)...)
	}
	return ids
}

// Window is a time field like activity_template.time: "always", "stop", or
// ["timer", [[y,m,d],[h,m,s]], [[y,m,d],[h,m,s]]] in server time. Anything
// else decodes as never open.
type Window struct {
	Always     bool
	Start, End time.Time // zero without a timer
}

// UnmarshalJSON implements json.Unmarshaler; it never fails, so a row is
// never skipped for its window.
func (window *Window) UnmarshalJSON(raw []byte) error {
	*window = Window{}
	var always string
	if json.Unmarshal(raw, &always) == nil {
		window.Always = always == "always"
		return nil
	}
	var timer []json.RawMessage
	if json.Unmarshal(raw, &timer) != nil || len(timer) < 3 {
		return nil
	}
	start, okStart := clock(timer[1])
	end, okEnd := clock(timer[2])
	if okStart && okEnd {
		window.Start, window.End = start, end
	}
	return nil
}

// clock reads a [[y,m,d],[h,m,s]] time.
func clock(raw json.RawMessage) (time.Time, bool) {
	var parts [2][3]int
	if json.Unmarshal(raw, &parts) != nil {
		return time.Time{}, false
	}
	return time.Date(parts[0][0], time.Month(parts[0][1]), parts[0][2], parts[1][0], parts[1][1], parts[1][2], 0, Zone), true
}

// Open is whether the window contains now.
func (window Window) Open(now time.Time) bool {
	if window.Always {
		return true
	}
	return !window.Start.IsZero() && !now.Before(window.Start) && now.Before(window.End)
}
