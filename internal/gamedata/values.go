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
// ["timer", [[y,m,d],[h,m,s]], [[y,m,d],[h,m,s]]] in server time, the label
// optional (shop_template.time is [start, end]; TimeMgr:inTime, mgr/timemgr.lua,
// reads both). Anything else decodes as never open.
//
// The new commander activities write [["newuser", ...], ["timer", ...]]:
// the timer goes in Start/End and the first part in NewUser. Such a window
// is never Open by itself, it opens for each commander (NewUser.Span).
type Window struct {
	Always     bool
	Start, End time.Time // zero without a timer
	NewUser    *NewUser
}

// NewUser is a window's ["newuser", offset days, days, [[y,m,d],[h,m,s]]]
// part (or ["newuser", [[y,m,d],[h,m,s]]]): the activity runs for each
// commander registered since Since, Days days long. Activity:getStartTime
// (model/vo/activity.lua) reads it back as ACTIVITYINFO.stop_time - Days.
type NewUser struct {
	Since        time.Time
	Offset, Days int
}

// parseNewUser reads a ["newuser", offset, days, since] or
// ["newuser", since] window part.
func parseNewUser(raw json.RawMessage) (NewUser, bool) {
	var parts []json.RawMessage
	var label string
	if json.Unmarshal(raw, &parts) != nil || len(parts) < 2 || json.Unmarshal(parts[0], &label) != nil || label != "newuser" {
		return NewUser{}, false
	}
	var newUser NewUser
	if len(parts) >= 4 && (json.Unmarshal(parts[1], &newUser.Offset) != nil || json.Unmarshal(parts[2], &newUser.Days) != nil) {
		return NewUser{}, false
	}
	since, ok := clock(parts[len(parts)-1])
	newUser.Since = since
	return newUser, ok
}

// Span is when the activity runs for a commander registered at registered:
// from the server day it registered (plus Offset days) for Days days, or
// up to timerEnd when the window gives no length; ok is false for a
// commander registered before Since.
func (newUser NewUser) Span(registered, timerEnd time.Time) (start, end time.Time, ok bool) {
	if registered.Before(newUser.Since) {
		return time.Time{}, time.Time{}, false
	}
	local := registered.In(Zone)
	start = time.Date(local.Year(), local.Month(), local.Day()+newUser.Offset, 0, 0, 0, 0, Zone)
	end = timerEnd
	if newUser.Days > 0 {
		end = start.AddDate(0, 0, newUser.Days)
	}
	return start, end, true
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
	if json.Unmarshal(raw, &timer) != nil {
		return nil
	}
	if len(timer) == 2 {
		if newUser, ok := parseNewUser(timer[0]); ok {
			window.UnmarshalJSON(timer[1])
			window.NewUser = &newUser
			return nil
		}
	}
	var label string
	if len(timer) > 0 && json.Unmarshal(timer[0], &label) == nil {
		timer = timer[1:]
	}
	if len(timer) < 2 {
		return nil
	}
	start, okStart := clock(timer[0])
	end, okEnd := clock(timer[1])
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
	if window.NewUser != nil {
		return false
	}
	if window.Always {
		return true
	}
	return !window.Start.IsZero() && !now.Before(window.Start) && now.Before(window.End)
}
