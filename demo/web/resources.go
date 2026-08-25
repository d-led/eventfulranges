package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
)

const dist = "dist"

// DoEmbed is set to true by the embed build tag (see embed.go).
var DoEmbed bool

//go:embed dist/*
var embeddedDist embed.FS

// GetFS returns the embedded UI when built with -tags embed, or the local
// dist/ directory during development.
func GetFS() http.FileSystem {
	if DoEmbed {
		sub, err := fs.Sub(embeddedDist, dist)
		if err != nil {
			panic(err)
		}
		return http.FS(sub)
	}
	return http.FS(os.DirFS(dist))
}
