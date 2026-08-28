// Command automerge is the same convergence story as the other demos — two
// replicas edit a shared document concurrently and sync to the same state —
// but the CRDT underneath is Automerge (automerge-go, a cgo binding to
// automerge-rs) instead of the eventfulranges range CRDT. It shows an
// alternative "strategy" for collaborative state: a JSON document with
// built-in conflict resolution, rather than a range set.
package main

import (
	"fmt"
	"sort"

	"github.com/automerge/automerge-go"
)

// run is the whole demo: two replicas fork from a shared document, edit
// different keys concurrently, sync, and print that they converged.
func run() {
	a := automerge.New()
	must("set title", a.RootMap().Set("title", "shopping list"))

	_, err := a.Commit("init")
	must("commit init", err)

	b, err := a.Fork()
	must("fork", err)

	must("set alice", a.RootMap().Set("alice", "apples"))

	_, err = a.Commit("alice adds apples")
	must("commit alice", err)

	must("set bob", b.RootMap().Set("bob", "bananas"))

	_, err = b.Commit("bob adds bananas")
	must("commit bob", err)

	syncPair(a, b)

	fmt.Println("alice's document:")
	printDoc(a)
	fmt.Println()
	fmt.Println("bob's document:")
	printDoc(b)
	fmt.Println()
	fmt.Printf("converged: %v\n", equalDocs(a, b))
}

// syncPair runs the Automerge sync protocol between two replicas until they
// converge.
func syncPair(a, b *automerge.Doc) {
	sa := automerge.NewSyncState(a)
	sb := automerge.NewSyncState(b)
	for {
		msg, ok := sa.GenerateMessage()
		if !ok {
			break
		}
		_, err := sb.ReceiveMessage(msg.Bytes())
		must("receive sync message", err)

		reply, ok := sb.GenerateMessage()
		if !ok {
			break
		}
		_, err = sa.ReceiveMessage(reply.Bytes())
		must("receive sync reply", err)
	}
}

// docEntries reads the whole root map as string values, so two replicas can be
// compared key-for-key.
func docEntries(d *automerge.Doc) map[string]string {
	keys, err := d.RootMap().Keys()
	must("read keys", err)

	out := make(map[string]string, len(keys))
	for _, k := range keys {
		v, err := automerge.As[string](d.RootMap().Get(k))
		must("read value", err)
		out[k] = v
	}
	return out
}

func equalDocs(a, b *automerge.Doc) bool {
	ea, eb := docEntries(a), docEntries(b)
	if len(ea) != len(eb) {
		return false
	}
	for k, v := range ea {
		if eb[k] != v {
			return false
		}
	}
	return true
}

func printDoc(d *automerge.Doc) {
	entries := docEntries(d)
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %s = %s\n", k, entries[k])
	}
}

// must aborts the demo with what on error. Demos crash loudly on purpose;
// production code should propagate errors instead.
func must(what string, err error) {
	if err != nil {
		panic(fmt.Errorf("%s: %w", what, err))
	}
}

func main() {
	run()
}
