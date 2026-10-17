package game

import (
	"errors"

	"google.golang.org/protobuf/proto"
)

// refusal turns a request down: the request is still answered, its reply
// carries a non-zero result (the client shows ERROR_MESSAGE[result] or its
// own tip) instead of the connection being closed.
type refusal struct {
	reason string
	code   uint32
}

func (r refusal) Error() string { return r.reason }

// resultFailed is the generic failure result.
const resultFailed = 1

var (
	// errRefused rejects a request the client shouldn't have sent (unknown
	// id, someone else's ship, an activity that isn't running...).
	errRefused error = refusal{"refused", resultFailed}
	// ErrNotEnough is returned when a commander can't pay for something.
	ErrNotEnough error = refusal{"not enough", resultFailed}
)

// refused tells whether err only means the request is turned down.
func refused(err error) bool {
	var r refusal
	return errors.As(err, &r)
}

// result is the result field of a reply to a request that ended with err
// (nil or a refusal): 0 when it went through, the refusal's code otherwise.
func result(err error) *uint32 {
	var r refusal
	switch {
	case err == nil:
		return proto.Uint32(0)
	case errors.As(err, &r):
		return proto.Uint32(r.code)
	}
	return proto.Uint32(resultFailed)
}
