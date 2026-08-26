package collab

import (
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

// StaticFS resolves the built UI filesystem, falling back to the "not built
// yet" placeholder when the built UI is missing. When embed is true the
// embedded filesystems are used; otherwise the dist directory on disk is.
// distDir is the demo's own generated directory, next to its sources.
func StaticFS(embed bool, embeddedDist, embeddedPlaceholder fs.FS, distDir string) http.FileSystem {
	if embed {
		if fsys := builtEmbedded(embeddedDist); fsys != nil {
			return fsys
		}
	} else if _, err := os.Stat(filepath.Join(distDir, "index.html")); err == nil {
		return http.Dir(distDir)
	}
	return placeholderFS(embeddedPlaceholder)
}

// builtEmbedded returns the embedded dist filesystem when it contains a built
// index.html, or nil when the UI has not been generated yet.
func builtEmbedded(embeddedDist fs.FS) http.FileSystem {
	sub, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return http.FS(sub)
}

// placeholderFS returns the embedded "not built yet" page.
func placeholderFS(embeddedPlaceholder fs.FS) http.FileSystem {
	sub, err := fs.Sub(embeddedPlaceholder, "placeholder")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}

// UIURL renders a clickable URL for the listen address, defaulting the host to
// localhost when the address names only a port.
func UIURL(addr string) string {
	host, port := "localhost", addr
	if h, p, err := net.SplitHostPort(addr); err == nil {
		host, port = h, p
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "localhost"
	}
	return "http://" + host + ":" + port + "/ui/"
}
