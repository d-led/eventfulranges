// Command go-automerge is the same convergence story as the other demos — two
// replicas edit a shared document concurrently and sync to the same state —
// but the CRDT underneath is a pure-Go port of Automerge
// (github.com/develerltd/go-automerge) instead of the eventfulranges range
// CRDT. Unlike the sibling automerge demo, this one needs no cgo.
package main

import (
	"fmt"
	"sort"

	am "github.com/develerltd/go-automerge/automerge"
)

// run is the whole demo: two replicas fork from a shared document, edit
// different keys concurrently, sync, and print that they converged.
func run() {
	a := am.New()
	must("put title", a.Put(am.Root, "title", am.NewStr("shopping list")))
	a.Commit("init", 1000)

	b := a.Fork()

	must("put alice", a.Put(am.Root, "alice", am.NewStr("apples")))
	a.Commit("alice adds apples", 2000)

	must("put bob", b.Put(am.Root, "bob", am.NewStr("bananas")))
	b.Commit("bob adds bananas", 2000)

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
func syncPair(a, b *am.Doc) {
	sa, sb := am.NewSyncState(), am.NewSyncState()
	for range 10 {
		sentA := relay(a, b, sa, sb)
		sentB := relay(b, a, sb, sa)
		if !sentA && !sentB {
			return
		}
	}
	panic("replicas did not converge after 10 rounds")
}

// relay forwards one pending change from src to dst, reporting whether src
// still had something to send.
func relay(src, dst *am.Doc, ss, ds *am.SyncState) bool {
	m := src.GenerateSyncMessage(ss)
	if m == nil {
		return false
	}
	must("receive sync message", dst.ReceiveSyncMessage(ds, m))
	return true
}

// docEntries reads the whole root map as string values, so two replicas can be
// compared key-for-key.
func docEntries(d *am.Doc) map[string]string {
	out := make(map[string]string)
	for _, k := range d.Keys(am.Root) {
		v, _, err := d.Get(am.Root, am.MapProp(k))
		if err != nil {
			panic(err)
		}
		out[k] = v.Scalar.Str()
	}
	return out
}

func equalDocs(a, b *am.Doc) bool {
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

func printDoc(d *am.Doc) {
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
