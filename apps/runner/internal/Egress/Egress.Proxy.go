package egress

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	statusBadRequest = http.StatusBadRequest
	statusForbidden  = http.StatusForbidden
	statusBadGateway = http.StatusBadGateway
	statusBusy       = http.StatusServiceUnavailable
	copyBufferBytes  = 32 << 10
)

var hopByHop = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Upgrade",
}

type headerLimit struct {
	r    io.Reader
	left int64
	open atomic.Bool
}

func (h *headerLimit) Read(b []byte) (int, error) {
	if h.open.Load() {
		return h.r.Read(b)
	}
	if h.left <= 0 {
		return 0, errHeaderTooLarge
	}
	if int64(len(b)) > h.left {
		b = b[:h.left]
	}
	n, err := h.r.Read(b)
	h.left -= int64(n)
	return n, err
}

var errHeaderTooLarge = fmt.Errorf("the request head is over %d bytes", maxHeaderBytes)

func (p *Proxy) handle(ctx context.Context, c net.Conn) {
	defer func() { _ = c.Close() }()
	_ = c.SetReadDeadline(time.Now().Add(p.o.HeaderTimeout))
	limit := &headerLimit{r: c, left: maxHeaderBytes}
	br := bufio.NewReader(limit)
	req, err := http.ReadRequest(br)
	if err != nil {
		refuse(c, statusBadRequest, "the proxy request could not be read")
		return
	}
	limit.open.Store(true)
	_ = c.SetReadDeadline(time.Time{})
	switch {
	case req.Method == http.MethodConnect:
		p.tunnel(ctx, c, br, req)
	case req.URL.IsAbs() && req.URL.Scheme == "http":
		p.forward(ctx, c, br, req)
	case req.URL.IsAbs():
		refuse(c, statusBadRequest, "use CONNECT for "+req.URL.Scheme+" URLs")
	default:
		refuse(c, statusBadRequest, "only CONNECT and absolute http:// requests go through the egress proxy")
	}
}

func (p *Proxy) tunnel(ctx context.Context, c net.Conn, br *bufio.Reader, req *http.Request) {
	host, port, ok := splitTarget(req.RequestURI, 0)
	if !ok {
		refuse(c, statusBadRequest, "CONNECT needs host:port")
		return
	}
	upstream, ok := p.open(ctx, c, host, port, true)
	if !ok {
		return
	}
	defer func() { _ = upstream.Close() }()
	if _, err := io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	p.splice(c, br, upstream)
}

func (p *Proxy) forward(ctx context.Context, c net.Conn, br *bufio.Reader, req *http.Request) {
	host, port, ok := splitTarget(req.URL.Host, 80)
	if !ok {
		refuse(c, statusBadRequest, "the URL has no valid host")
		return
	}
	upstream, ok := p.open(ctx, c, host, port, false)
	if !ok {
		return
	}
	defer func() { _ = upstream.Close() }()
	for _, h := range connectionTokens(req.Header) {
		req.Header.Del(h)
	}
	for _, h := range hopByHop {
		req.Header.Del(h)
	}
	if strings.EqualFold(req.Header.Get("Expect"), "100-continue") {
		req.Header.Del("Expect")
		if _, err := io.WriteString(c, "HTTP/1.1 100 Continue\r\n\r\n"); err != nil {
			return
		}
	}
	req.Close = true
	req.RequestURI = ""
	_ = upstream.SetWriteDeadline(time.Now().Add(p.o.Idle))
	if err := req.Write(upstream); err != nil {
		refuse(c, statusBadGateway, "the request could not be sent upstream")
		return
	}
	_ = upstream.SetWriteDeadline(time.Time{})
	p.splice(c, br, upstream)
}

func (p *Proxy) open(ctx context.Context, c net.Conn, host string, port int, tunnel bool) (net.Conn, bool) {
	name, why := p.decide(host, port, tunnel)
	if why != "" {
		p.report(name, port, why)
		refuse(c, statusForbidden, why)
		return nil, false
	}
	upstream, why, status := p.dial(ctx, name, port)
	if upstream == nil {
		p.report(name, port, why)
		refuse(c, status, why)
		return nil, false
	}
	p.report(name, port, "")
	return upstream, true
}

func (p *Proxy) splice(client net.Conn, fromClient io.Reader, upstream net.Conn) {
	var last atomic.Int64
	last.Store(time.Now().UnixNano())
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() { p.copy(upstream, fromClient, &last) })
	wg.Go(func() { p.copy(client, upstream, &last) })
	go p.watchIdle(&last, stop, client, upstream)
	wg.Wait()
	close(stop)
}

func (p *Proxy) copy(dst net.Conn, src io.Reader, last *atomic.Int64) {
	buf := make([]byte, copyBufferBytes)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			last.Store(time.Now().UnixNano())
			if _, werr := dst.Write(buf[:n]); werr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = dst.Close()
}

func (p *Proxy) watchIdle(last *atomic.Int64, stop <-chan struct{}, conns ...net.Conn) {
	tick := time.NewTicker(max(p.o.Idle/4, 10*time.Millisecond))
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-tick.C:
			if now.Sub(time.Unix(0, last.Load())) > p.o.Idle {
				for _, c := range conns {
					_ = c.Close()
				}
				return
			}
		}
	}
}

func refuse(c net.Conn, status int, why string) {
	body := "dhow egress: " + why + "\n"
	_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, _ = fmt.Fprintf(c, "HTTP/1.1 %d %s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
		status, http.StatusText(status), len(body), body)
}

func splitTarget(hostport string, defaultPort int) (string, int, bool) {
	host, portText, err := net.SplitHostPort(hostport)
	if err != nil {
		if defaultPort == 0 || strings.Contains(hostport, ":") {
			return "", 0, false
		}
		host, portText = hostport, strconv.Itoa(defaultPort)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != portText || host == "" {
		return "", 0, false
	}
	return host, port, true
}

func connectionTokens(h http.Header) []string {
	var out []string
	for _, v := range h.Values("Connection") {
		for _, t := range strings.Split(v, ",") {
			if t = strings.TrimSpace(t); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}
