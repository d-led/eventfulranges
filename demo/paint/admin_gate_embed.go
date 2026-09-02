//go:build embed

package main

import (
	"os"

	"github.com/d-led/eventfulranges/demo/internal/collab"
)

// adminGate returns the gate guarding the admin area in a deployed (embedded)
// build. Only the ADMIN_EMAILS allow-list passes; a request that carries no
// reverse-proxy email is never admitted, so the area stays closed unless the
// operator configures the secret. The development-only open gate lives in
// admin_gate_dev.go, which this build tag excludes from the artifact.
func adminGate() collab.AdminGate {
	return adminGateFor(os.Getenv("ADMIN_EMAILS"))
}

// adminGateFor resolves the ADMIN_EMAILS setting to a gate: the allow-list
// and nothing else, for an embedded build.
func adminGateFor(emails string) collab.AdminGate {
	return parseAdminList(emails)
}
