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

func newPeer() (*peer, error) {
	set, err := eventfulranges.OpenStore(context.Background(), memory.New(), strategy.LWW)
	if err != nil {
		return nil, err
	}
	return &peer{set: set}, nil
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
func (p *peer) start(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	p.addr = ln.Addr().String()
	p.server = &http.Server{Handler: p.handler()}
	go func() { _ = p.server.Serve(ln) }()
	return nil
}

func (p *peer) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = p.server.Shutdown(ctx)
}

// exchange pushes this peer's operations to the other peer.
func (p *peer) exchange(ctx context.Context, other string) error {
	body, err := json.Marshal(p.set.Ops())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+other+"/ops", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
}

func run(ports []int) error {
	ctx := context.Background()
	a, err := startPeer(ports[0])
	if err != nil {
		return err
	}
	defer a.stop()
	b, err := startPeer(ports[1])
	if err != nil {
		return err
	}
	defer b.stop()

	if err := mutate(ctx, a.set, b.set); err != nil {
		return err
	}
	if err := exchangeBoth(ctx, a, b); err != nil {
		return err
	}
	return reportConvergence(a.set, b.set)
}

// startPeer creates and starts a peer on the given port.
func startPeer(port int) (*peer, error) {
	p, err := newPeer()
	if err != nil {
		return nil, err
	}
	if err := p.start(port); err != nil {
		return nil, err
	}
	return p, nil
}

// mutate gives each peer its own independent edits.
func mutate(ctx context.Context, a, b *eventfulranges.RangeSet) error {
	if _, err := a.Add(ctx, 1, 8); err != nil {
		return err
	}
	if _, err := a.Remove(ctx, 3, 4); err != nil {
		return err
	}
	_, err := b.Add(ctx, 6, 10)
	return err
}

// exchangeBoth runs anti-entropy over HTTP in both directions.
func exchangeBoth(ctx context.Context, a, b *peer) error {
	if err := a.exchange(ctx, b.addr); err != nil {
		return err
	}
	return b.exchange(ctx, a.addr)
}

// reportConvergence fails if the peers diverged, otherwise prints the result.
func reportConvergence(a, b *eventfulranges.RangeSet) error {
	if !interval.Equal(a.Ranges(), b.Ranges()) {
		return fmt.Errorf("peers did not converge: %v vs %v", a.Ranges(), b.Ranges())
	}
	fmt.Println("converged to:", a.Ranges())
	return nil
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
	if err := run(ports); err != nil {
		log.Fatal(err)
	}
}
