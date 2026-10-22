package admin

import (
	"net/http"
	"strconv"

	"github.com/ggmolly/belfast/internal/logger"
)

// The logs screen: the logger's recent events. The page sorts them itself
// (missing handlers, refusals...) from the stream.

type logEvent struct {
	Seq     uint64      `json:"seq"`
	Time    int64       `json:"time"` // unix milliseconds
	Level   string      `json:"level"`
	Scope   string      `json:"scope"`
	Fields  [][2]string `json:"fields"`
	Message string      `json:"message"`
}

type logsReply struct {
	// the events after ?after, and the number to ask after next time
	Events []logEvent `json:"events"`
	Next   uint64     `json:"next"`
}

// handleLogs is the recent log events after ?after= (a seq, 0 for all the
// logger keeps).
func (server *Server) handleLogs(r *http.Request) (any, error) {
	var after uint64
	if raw := r.URL.Query().Get("after"); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, badRequest("after: " + err.Error())
		}
		after = n
	}
	reply := logsReply{Events: []logEvent{}, Next: after}
	for _, event := range logger.Recent(after) {
		fields := make([][2]string, 0, len(event.Fields))
		for _, field := range event.Fields {
			if field.Key != "" && field.Key != "event" {
				fields = append(fields, [2]string{field.Key, field.Value})
			}
		}
		reply.Events = append(reply.Events, logEvent{
			Seq: event.Seq, Time: event.Time.UnixMilli(), Level: logger.LevelName(event.Level),
			Scope: event.Scope, Fields: fields, Message: event.Message,
		})
		reply.Next = event.Seq
	}
	return reply, nil
}
