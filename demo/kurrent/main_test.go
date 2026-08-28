//go:build kurrent

package main

import (
	"os"
	"testing"
)

func TestRun(t *testing.T) {
	conn := os.Getenv("KURRENTDB_CONNECTION")
	if conn == "" {
		t.Skip("KURRENTDB_CONNECTION not set; run kurrent-up.sh first")
	}
	run(conn)
}
