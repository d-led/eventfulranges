package main

import (
	"flag"
	"log"

	"github.com/gin-gonic/gin"

	"github.com/d-led/eventfulranges/demo/internal/collab"
)

//go:generate npm --prefix ui-src install --no-audit --no-fund
//go:generate npm --prefix ui-src run build

func main() {
	addr := flag.String("addr", ":8081", "listen address")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	sessions := collab.NewSessions(collab.SessionTTL, func() collab.Model { return newBoard() })
	log.Printf("eventfulranges whiteboard listening on %s", *addr)
	log.Printf("open %s", collab.UIURL(*addr))
	if err := collab.NewRouter(sessions, GetFS(), []string{"d"}).Run(*addr); err != nil {
		log.Fatal(err)
	}
}
