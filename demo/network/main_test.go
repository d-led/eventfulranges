package main

import "testing"

func TestRunConverges(t *testing.T) {
	t.Parallel()
	// Port 0 asks the kernel for free ephemeral ports.
	run([]int{0, 0})
}
