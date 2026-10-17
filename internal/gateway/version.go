package gateway

import (
	"fmt"
	"net"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

const upstreamTimeout = 10 * time.Second

// version returns the upstream SC_10801 for this request, cached per
// (state, platform). A stale copy is served when the upstream is unreachable.
func (gateway *Gateway) version(req *protobuf.CS_10800) (*protobuf.SC_10801, error) {
	key := fmt.Sprintf("%d/%s", req.GetState(), req.GetPlatform())
	ttl := gateway.VersionTTL
	if ttl == 0 {
		ttl = DefaultVersionTTL
	}
	gateway.mu.Lock()
	cached, ok := gateway.versions[key]
	gateway.mu.Unlock()
	if ok && time.Since(cached.at) < ttl {
		return cached.msg, nil
	}
	msg, err := fetchVersion(gateway.Upstream, req)
	if err != nil {
		if ok {
			logger.LogEvent("Gateway", "Version", fmt.Sprintf("upstream %s: %v, serving cached reply from %s", gateway.Upstream, err, cached.at.Format(time.RFC3339)), logger.LOG_LEVEL_WARN)
			return cached.msg, nil
		}
		return nil, fmt.Errorf("upstream %s: %w", gateway.Upstream, err)
	}
	gateway.mu.Lock()
	if gateway.versions == nil {
		gateway.versions = make(map[string]cachedVersion)
	}
	gateway.versions[key] = cachedVersion{msg: msg, at: time.Now()}
	gateway.mu.Unlock()
	return msg, nil
}

// fetchVersion sends req to the gateway at address and reads its SC_10801.
func fetchVersion(address string, req *protobuf.CS_10800) (*protobuf.SC_10801, error) {
	reqID, err := protocol.ClientPacketID(req)
	if err != nil {
		return nil, err
	}
	payload, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", address, upstreamTimeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(upstreamTimeout))
	if _, err := conn.Write(protocol.AppendFrame(nil, reqID, 0, payload)); err != nil {
		return nil, err
	}
	frame, err := protocol.ReadFrame(conn)
	if err != nil {
		return nil, err
	}
	msg := &protobuf.SC_10801{}
	wantID, err := protocol.ServerPacketID(msg)
	if err != nil {
		return nil, err
	}
	if id := protocol.ParseHeader(frame).ID; id != wantID {
		return nil, fmt.Errorf("got SC_%d, want SC_%d", id, wantID)
	}
	if err := proto.Unmarshal(frame[protocol.HeaderSize:], msg); err != nil {
		return nil, err
	}
	return msg, nil
}
