//go:build chaos

package chaos_test

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type faultyProxy struct {
	ln       net.Listener
	target   string
	mu       sync.Mutex
	conns    map[net.Conn]struct{}
	blocked  atomic.Bool
	throttle atomic.Int64
}

func newFaultyProxy(listen, target string) (*faultyProxy, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", listen)
	if err != nil {
		return nil, err
	}
	p := &faultyProxy{ln: ln, target: target, conns: map[net.Conn]struct{}{}}
	go p.serve()
	return p, nil
}

func (p *faultyProxy) Close() {
	_ = p.ln.Close()
	p.dropAll()
}

func (p *faultyProxy) Addr() string { return p.ln.Addr().String() }

func (p *faultyProxy) serve() {
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return
		}
		if p.blocked.Load() {
			go p.hold(c)
			continue
		}
		go p.pipe(c)
	}
}

func (p *faultyProxy) hold(c net.Conn) {
	p.track(c)
	_, _ = io.Copy(io.Discard, c)
	p.untrack(c)
	_ = c.Close()
}

func (p *faultyProxy) pipe(c net.Conn) {
	var d net.Dialer
	up, err := d.DialContext(context.Background(), "tcp", p.target)
	if err != nil {
		_ = c.Close()
		return
	}
	p.track(c)
	p.track(up)
	var wg sync.WaitGroup
	wg.Go(func() { p.copy(up, c) })
	p.copy(c, up)
	wg.Wait()
	p.untrack(c)
	p.untrack(up)
}

func (p *faultyProxy) copy(dst, src net.Conn) {
	buf := make([]byte, 16<<10)
	for {
		if bps := p.throttle.Load(); bps > 0 {
			buf = buf[:min(int64(cap(buf)), max(bps/10, 1))]
		} else {
			buf = buf[:cap(buf)]
		}
		n, err := src.Read(buf)
		if n > 0 {
			for p.blocked.Load() {
				time.Sleep(50 * time.Millisecond)
			}
			if _, werr := dst.Write(buf[:n]); werr != nil {
				break
			}
			if p.throttle.Load() > 0 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		if err != nil {
			break
		}
	}
	_ = dst.Close()
	_ = src.Close()
}

func (p *faultyProxy) track(c net.Conn) {
	p.mu.Lock()
	p.conns[c] = struct{}{}
	p.mu.Unlock()
}

func (p *faultyProxy) untrack(c net.Conn) {
	p.mu.Lock()
	delete(p.conns, c)
	p.mu.Unlock()
}

func (p *faultyProxy) dropAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for c := range p.conns {
		_ = c.Close()
	}
}

func (p *faultyProxy) Blackhole(on bool) { p.blocked.Store(on) }

func (p *faultyProxy) Throttle(bytesPerSecond int64) { p.throttle.Store(bytesPerSecond) }
