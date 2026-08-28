// Command network demonstrates convergence over HTTP: two peers each mutate
// their own replica and then exchange operations with plain POST requests
// until both hold the same set. No CRDT-specific protocol is needed — the
// peers just ship their operation logs to each other.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/d-led/eventfulranges"
	"github.com/d-led/eventfulranges/interval"
	"github.com/d-led/eventfulranges/op"
	"github.com/d-led/eventfulranges/store/memory"
	"github.com/d-led/eventfulranges/strategy"
)

// peer is a range set served over HTTP.
type peer struct {
	set    *eventfulranges.RangeSet
	server *http.Server
	addr   string
}

func newPeer() *peer {
	set, err := eventfulranges.OpenStore(context.Background(), memory.New(), strategy.LWW)
	must("open the peer set", err)
	return &peer{set: set}
}

func (p *peer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ops", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(p.set.Ops())
	})
	mux.HandleFunc("POST /ops", func(w http.ResponseWriter, r *http.Request) {
		var ops []op.Op
		if err := json.NewDecoder(r.Body).Decode(&ops); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := p.set.ApplyAll(r.Context(), ops); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

// start listens on 127.0.0.1:port and serves until stop is called. Port 0
// picks a free ephemeral port, which the tests use.
func (p *peer) start(port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	must("listen on port", err)
	p.addr = ln.Addr().String()
	p.server = &http.Server{Handler: p.handler()}
	go func() { _ = p.server.Serve(ln) }()
}

func (p *peer) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = p.server.Shutdown(ctx)
}

// exchange pushes this peer's operations to the other peer.
func (p *peer) exchange(ctx context.Context, other string) {
	body, err := json.Marshal(p.set.Ops())
	must("marshal ops", err)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+other+"/ops", bytes.NewReader(body))
	must("build request", err)

	resp, err := http.DefaultClient.Do(req)
	must("send ops", err)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		panic(fmt.Errorf("unexpected status %s", resp.Status))
	}
}

// run starts two peers, mutates each, and exchanges ops until they converge.
func run(ports []int) {
	ctx := context.Background()
	a := startPeer(ports[0])
	defer a.stop()
	b := startPeer(ports[1])
	defer b.stop()

	mutate(ctx, a.set, b.set)
	exchangeBoth(ctx, a, b)
	reportConvergence(a.set, b.set)
}

// startPeer creates and starts a peer on the given port.
func startPeer(port int) *peer {
	p := newPeer()
	p.start(port)
	return p
}

// mutate gives each peer its own independent edits.
func mutate(ctx context.Context, a, b *eventfulranges.RangeSet) {
	_, err := a.Add(ctx, 1, 8)
	must("add [1,8]", err)

	_, err = a.Remove(ctx, 3, 4)
	must("remove [3,4]", err)

	_, err = b.Add(ctx, 6, 10)
	must("add [6,10]", err)
}

// exchangeBoth runs anti-entropy over HTTP in both directions.
func exchangeBoth(ctx context.Context, a, b *peer) {
	a.exchange(ctx, b.addr)
	b.exchange(ctx, a.addr)
}

// reportConvergence prints the shared set, panicking if the peers diverged.
func reportConvergence(a, b *eventfulranges.RangeSet) {
	if !interval.Equal(a.Ranges(), b.Ranges()) {
		panic(fmt.Errorf("peers did not converge: %v vs %v", a.Ranges(), b.Ranges()))
	}
	fmt.Println("converged to:", a.Ranges())
}

// must aborts the demo with what on error. Demos crash loudly on purpose;
// production code should propagate errors instead.
func must(what string, err error) {
	if err != nil {
		panic(fmt.Errorf("%s: %w", what, err))
	}
}

func main() {
	portsFlag := flag.String("ports", "18080,18081", "comma-separated listen ports for the two peers")
	flag.Parse()

	var ports []int
	for _, part := range strings.Split(*portsFlag, ",") {
		p, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			log.Fatal(err)
		}
		ports = append(ports, p)
	}
	if len(ports) != 2 {
		log.Fatal("need exactly two ports")
	}
	run(ports)
}
