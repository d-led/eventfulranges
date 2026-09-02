//go:build !embed

package main

import (
	"os"

	"github.com/d-led/eventfulranges/demo/internal/collab"
)

// devOpenGate admits requests that carry no reverse-proxy email — that is,
// every request to a server run without oauth2-proxy in front of it. A local
// developer's browser is such a request, so the admin area is reachable while
// developing. Requests that do name an identity are still refused unless the
// ADMIN_EMAILS allow-list admits them.
type devOpenGate struct{}

// IsAdmin reports whether the given (proxy) email may use the admin area.
func (devOpenGate) IsAdmin(email string) bool { return email == "" }

// adminGate returns the gate guarding the admin area in a development
// (non-embedded) build. A configured ADMIN_EMAILS allow-list behaves exactly
// as in the deployed build; without one, direct requests are admitted so the
// admin area is usable while developing.
func adminGate() collab.AdminGate {
	return adminGateFor(os.Getenv("ADMIN_EMAILS"))
}

// adminGateFor resolves the ADMIN_EMAILS setting to a gate. With a list, only
// listed emails pass; without one, the area is open to direct requests.
func adminGateFor(emails string) collab.AdminGate {
	if emails != "" {
		return parseAdminList(emails)
	}
	return devOpenGate{}
}
