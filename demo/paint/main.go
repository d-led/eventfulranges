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
	data := flag.String("data", "data", "directory for persistent sessions (empty disables)")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	factory := func() collab.Model { return newBoard() }
	var sessions *collab.Sessions
	if *data == "" {
		sessions = collab.NewSessions(collab.SessionTTL, factory)
	} else {
		sessions = collab.NewPersistentSessions(collab.SessionTTL, *data, factory)
	}
	log.Printf("eventfulranges whiteboard listening on %s", *addr)
	log.Printf("open %s", collab.UIURL(*addr))
	if err := collab.NewRouter(sessions, GetFS(), nil).Run(*addr); err != nil {
		log.Fatal(err)
	}
}
