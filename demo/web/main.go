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
	"path/filepath"

	"github.com/gin-gonic/gin"
)

//go:generate npm --prefix ui-src install --no-audit --no-fund
//go:generate npm --prefix ui-src run build

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	if err := ensureUI(); err != nil {
		log.Fatal(err)
	}

	s := newSessions(sessionTTL)
	log.Printf("eventfulranges visualizer listening on %s", *addr)
	log.Printf("open %s", uiURL(*addr))
	if err := newRouter(s, GetFS()).Run(*addr); err != nil {
		log.Fatal(err)
	}
}

// ensureUI fails with a hint when the dev-mode UI has not been built yet.
// Embedded builds always have the UI and skip the check.
func ensureUI() error {
	if DoEmbed {
		return nil
	}
	if _, err := os.Stat(filepath.Join(distDir(), "index.html")); err != nil {
		return fmt.Errorf("web UI not built yet — run `go generate ./demo/web` (or `npm --prefix demo/web/ui-src run build`)")
	}
	return nil
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
