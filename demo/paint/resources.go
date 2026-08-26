package main

import (
	"embed"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/d-led/eventfulranges/demo/internal/collab"
)

// DoEmbed is set to true by the embed build tag (see embed.go).
var DoEmbed bool

// The generated UI is embedded wholesale. all: includes the committed .gitkeep
// marker, so the pattern still matches on a fresh checkout where the UI has
// not been built yet.
//
//go:embed all:dist
var embeddedDist embed.FS

// The "not built yet" placeholder is always embedded, so the server can show
// build instructions instead of a 404 when dist/ has no index.html.
//
//go:embed placeholder/*
var embeddedPlaceholder embed.FS

// GetFS returns the built UI when it exists, and the placeholder otherwise.
func GetFS() http.FileSystem {
	return collab.StaticFS(DoEmbed, embeddedDist, embeddedPlaceholder, distDir())
}

// distDir resolves the dist directory relative to this source file.
func distDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "dist"
	}
	return filepath.Join(filepath.Dir(file), "dist")
}
