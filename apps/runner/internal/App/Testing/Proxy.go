//go:build dhowtest

package runnertest

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const copyBuffer = 32 << 10

type Proxy struct {
	Addr      string
	target    string
	ln        net.Listener
	blackhole atomic.Bool
	mu        sync.Mutex
	conns     []net.Conn
	wg        sync.WaitGroup
}

func NewProxy(t testing.TB, target string) *Proxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &Proxy{Addr: ln.Addr().String(), target: strings.TrimPrefix(target, "https://"), ln: ln}
	p.wg.Add(1)
	go p.accept()
	t.Cleanup(p.Close)
	return p
}

func (p *Proxy) URL() string { return "https://" + p.Addr }

func (p *Proxy) Blackhole(on bool) { p.blackhole.Store(on) }

func (p *Proxy) accept() {
	defer p.wg.Done()
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return
		}
		p.track(c)
		p.wg.Add(1)
		go p.serve(c)
	}
}

func (p *Proxy) track(c net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conns = append(p.conns, c)
}

func (p *Proxy) serve(client net.Conn) {
	defer p.wg.Done()
	if p.blackhole.Load() {
		_, _ = io.Copy(io.Discard, client)
		return
	}
	upstream, err := net.Dial("tcp", p.target)
	if err != nil {
		_ = client.Close()
		return
	}
	p.track(upstream)
	var done sync.WaitGroup
	done.Add(2)
	go func() { defer done.Done(); p.pipe(upstream, client) }()
	go func() { defer done.Done(); p.pipe(client, upstream) }()
	done.Wait()
}

func (p *Proxy) pipe(dst, src net.Conn) {
	buf := make([]byte, copyBuffer)
	for {
		n, err := src.Read(buf)
		if n > 0 && !p.blackhole.Load() {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				_ = dst.Close()
			}
			return
		}
	}
}

func (p *Proxy) Close() {
	_ = p.ln.Close()
	p.mu.Lock()
	for _, c := range p.conns {
		_ = c.Close()
	}
	p.mu.Unlock()
	p.wg.Wait()
}
