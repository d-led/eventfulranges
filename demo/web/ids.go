// Identifier minting shared by every build of the visualizer. The native
// server hands out one session id per share link and one client id per
// socket; the wasm build hands out one of each per page, so the activity log
// can attribute operations the same way on either side.
package main

import (
	"crypto/rand"
	"encoding/base32"
)

// newSessionID mints a short, URL-safe, unguessable identifier for a fresh
// share link.
func newSessionID() string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}

// newClientID mints a short, human-readable identity for one connected
// browser, so the activity log can attribute operations to collaborators.
func newClientID() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}
