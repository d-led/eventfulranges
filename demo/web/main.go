//go:build !js

// Command web serves an interactive n-dimensional range-set visualizer.
//
// Every share link carries a unique session ID; browsers that open the same
// link join the same model and converge over a WebSocket, while other links
// stay isolated. Each add or remove is folded with additive-wins semantics —
// the union of every addition minus the union of every removal — so concurrent
// edits from any number of users converge regardless of arrival order.
// Sessions expire from an in-memory cache after a day without use.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/gin-gonic/gin"
)

// Build metadata, injected by goreleaser with -ldflags at release time and
// left at these defaults for local development.
var (
	version = "dev"
	commit  = "unknown"
	branch  = "unknown"
)

//go:generate npm --prefix ui-src install --no-audit --no-fund
//go:generate npm --prefix ui-src run build

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	defaultData := os.Getenv("DATA_DIR")
	if defaultData == "" {
		defaultData = "data"
	}
	data := flag.String("data", defaultData, "directory for persistent sessions")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	s := newPersistentSessions(sessionTTL, *data)
	log.Printf("eventfulranges visualizer %s (branch %s, commit %s) listening on %s", version, branch, commit, *addr)
	log.Printf("open %s", uiURL(*addr))
	if err := newRouter(s, GetFS()).Run(*addr); err != nil {
		log.Fatal(err)
	}
}

// uiURL renders a clickable URL for the listen address, defaulting the host
// to localhost when the address names only a port.
func uiURL(addr string) string {
	host, port := "localhost", addr
	if h, p, err := net.SplitHostPort(addr); err == nil {
		host, port = h, p
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%s/ui/", host, port)
}
