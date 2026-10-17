// tap is a transparent capture proxy: game clients connect to it instead of
// the real server, every packet is relayed verbatim upstream and decoded into
// a JSONL capture for disassembly.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ggmolly/belfast/internal/tap"
)

func main() {
	listen := flag.String("listen", ":80", "listen address")
	upstream := flag.String("upstream", "", "upstream host:port to relay to (required)")
	capture := flag.String("capture", "capture.jsonl", "JSONL capture output path")
	flag.Parse()
	if *upstream == "" {
		fmt.Fprintln(os.Stderr, "tap: -upstream is required")
		os.Exit(1)
	}
	file, err := os.OpenFile(*capture, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tap: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()
	if err := (&tap.Tap{Listen: *listen, Upstream: *upstream, Sink: tap.NewSink(file)}).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tap: %v\n", err)
		os.Exit(1)
	}
}
