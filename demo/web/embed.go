//go:build embed && !js

package main

import "log"

func init() {
	log.Println("using embedded UI resources")
	DoEmbed = true
}
