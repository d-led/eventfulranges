package main

import (
	"flag"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/d-led/eventfulranges/demo/internal/collab"
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
	log.Printf("eventfulranges whiteboard listening on %s", *addr)
	log.Printf("open %s", collab.UIURL(*addr))
	if err := collab.NewRouter(sessions, GetFS(), nil).Run(*addr); err != nil {
		log.Fatal(err)
	}
}
