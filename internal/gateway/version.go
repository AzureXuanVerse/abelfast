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

// maxCachedVersions bounds the version cache: a client build sends one
// state and platform, more keys only come from clients making them up. Past
// it the oldest reply is dropped.
const maxCachedVersions = 16

// version returns the upstream SC_10801 for this request, cached per
// (state, platform). Concurrent requests for the same key share one upstream
// fetch. A stale copy is served when the upstream is unreachable.
func (gateway *Gateway) version(req *protobuf.CS_10800) (*protobuf.SC_10801, error) {
	key := fmt.Sprintf("%d/%s", req.GetState(), req.GetPlatform())
	gateway.mu.Lock()
	cached, ok := gateway.versions[key]
	if ok && time.Since(cached.at) < versionTTL {
		gateway.mu.Unlock()
		return cached.msg, nil
	}
	fetch, inFlight := gateway.fetching[key]
	if !inFlight {
		fetch = &versionFetch{done: make(chan struct{})}
		if gateway.fetching == nil {
			gateway.fetching = make(map[string]*versionFetch)
		}
		gateway.fetching[key] = fetch
	}
	gateway.mu.Unlock()

	if inFlight {
		<-fetch.done
	} else {
		fetch.msg, fetch.err = fetchVersion(gateway.Upstream, req)
		gateway.mu.Lock()
		delete(gateway.fetching, key)
		if fetch.err == nil {
			gateway.cacheVersion(key, fetch.msg)
		}
		gateway.mu.Unlock()
		close(fetch.done)
	}
	if fetch.err != nil {
		if ok {
			logger.LogEvent("Gateway", "Version", fmt.Sprintf("upstream %s: %v, serving cached reply from %s", gateway.Upstream, fetch.err, cached.at.Format(time.RFC3339)), logger.LOG_LEVEL_WARN)
			return cached.msg, nil
		}
		return nil, fmt.Errorf("upstream %s: %w", gateway.Upstream, fetch.err)
	}
	return fetch.msg, nil
}

// cacheVersion stores an upstream reply, dropping the oldest one when the
// cache is full. gateway.mu must be held.
func (gateway *Gateway) cacheVersion(key string, msg *protobuf.SC_10801) {
	if gateway.versions == nil {
		gateway.versions = make(map[string]cachedVersion)
	}
	if _, ok := gateway.versions[key]; !ok && len(gateway.versions) >= maxCachedVersions {
		oldest := ""
		for other, cached := range gateway.versions {
			if oldest == "" || cached.at.Before(gateway.versions[oldest].at) {
				oldest = other
			}
		}
		delete(gateway.versions, oldest)
	}
	gateway.versions[key] = cachedVersion{msg: msg, at: time.Now()}
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
