package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	return http.FS(os.DirFS(distDir()))
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
