package gateway

import (
	"time"

	"github.com/ggmolly/belfast/internal/protobuf"
)

// LatestVersion is the newest upstream SC_10801 in the cache and when it was
// relayed, nil before the first one. It never asks the upstream.
func (gateway *Gateway) LatestVersion() (*protobuf.SC_10801, time.Time) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	var latest cachedVersion
	for _, cached := range gateway.versions {
		if latest.msg == nil || cached.at.After(latest.at) {
			latest = cached
		}
	}
	return latest.msg, latest.at
}
