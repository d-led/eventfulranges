package main

import (
	"flag"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/d-led/eventfulranges/demo/internal/collab"
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
	addr := flag.String("addr", ":8081", "listen address")
	defaultData := os.Getenv("DATA_DIR")
	if defaultData == "" {
		defaultData = "data"
	}
	data := flag.String("data", defaultData, "directory for persistent sessions")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	sessions := collab.NewPersistentSessions(collab.SessionTTL, *data, func() collab.Model { return newBoard() })
	log.Printf("eventfulranges ± infinite paint %s (branch %s, commit %s) listening on %s", version, branch, commit, *addr)
	log.Printf("open %s", collab.UIURL(*addr))
	router := collab.NewRouter(sessions, GetFS(), nil)
	router.GET("/api/export", exportHandler(sessions))
	collab.RegisterAdminRoutes(router, sessions, parseAdminList(os.Getenv("ADMIN_EMAILS")), GetFS())
	if err := router.Run(*addr); err != nil {
		log.Fatal(err)
	}
}
