package gateway

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// PasswordHash is what inner_accounts.password_hash holds for a password,
// as the inner login checks it.
func PasswordHash(password string) []byte {
	return passwordHash(password)
}

// CheckUpstream asks the upstream gateway for the Android clients' SC_10801
// now, as AssetHash would, and caches the reply in place of the one it may
// still be reusing.
func (gateway *Gateway) CheckUpstream() (*protobuf.SC_10801, error) {
	gateway.mu.Lock()
	state := gateway.androidState
	gateway.mu.Unlock()
	if state == 0 {
		state = capturedState
	}
	req := &protobuf.CS_10800{State: proto.Uint32(state), Platform: proto.String(androidPlatform)}
	msg, err := fetchVersion(gateway.Upstream, req)
	if err != nil {
		return nil, fmt.Errorf("upstream %s: %w", gateway.Upstream, err)
	}
	gateway.mu.Lock()
	gateway.cacheVersion(fmt.Sprintf("%d/%s", req.GetState(), req.GetPlatform()), msg)
	gateway.mu.Unlock()
	return msg, nil
}
