// Command web serves an interactive n-dimensional range-set visualizer.
//
// Browsers connected to the same instance share one view. Each add or remove
// is folded with additive-wins semantics — the union of every addition minus
// the union of every removal — and broadcast over a WebSocket, so concurrent
// edits from any number of users converge regardless of arrival order.
package main

import (
	"flag"
	"log"

	"github.com/gin-gonic/gin"
)

//go:generate npm --prefix ui-src install --no-audit --no-fund
//go:generate npm --prefix ui-src run build

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	h := newHub()
	log.Printf("eventfulranges visualizer listening on %s", *addr)
	if err := newRouter(h, GetFS()).Run(*addr); err != nil {
		log.Fatal(err)
	}
}
