//go:build !js

package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const (
	dist        = "dist"
	placeholder = "placeholder"
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

// GetFS returns the built UI when it exists, and the "not built yet"
// placeholder otherwise.
func GetFS() http.FileSystem {
	if d := builtDist(); d != nil {
		return http.FS(d)
	}
	return placeholderFS()
}

// builtDist resolves the generated dist/ filesystem, or nil when the UI has
// not been built. Embedded builds read the embedded dist; dev builds read the
// dist/ directory next to this file.
func builtDist() fs.FS {
	if DoEmbed {
		sub, err := fs.Sub(embeddedDist, dist)
		if err != nil {
			panic(err)
		}
		if _, err := fs.Stat(sub, "index.html"); err != nil {
			return nil
		}
		return sub
	}
	if _, err := os.Stat(filepath.Join(distDir(), "index.html")); err != nil {
		return nil
	}
	return os.DirFS(distDir())
}

// placeholderFS returns the embedded "not built yet" page.
func placeholderFS() http.FileSystem {
	sub, err := fs.Sub(embeddedPlaceholder, placeholder)
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}

// distDir resolves the dist directory relative to this source file, so the
// server works no matter which directory it is started from.
func distDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return dist
	}
	return filepath.Join(filepath.Dir(file), dist)
}
