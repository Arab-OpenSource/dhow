package mailer_test

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"io"
	"math/big"
	"mime/quotedprintable"
	"net"
	netmail "net/mail"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mailer "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Mail"
)

type fakeServer struct {
	addr      string
	tlsConfig *tls.Config
	starttls  bool
	implicit  bool
	silent    bool
	reject    bool

	mu       sync.Mutex
	commands []string
	data     string
	auth     string
	tlsUsed  bool
}

func selfSigned(t *testing.T) (*tls.Config, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}, pool
}

func (f *fakeServer) start(t *testing.T) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	if f.implicit {
		ln = tls.NewListener(ln, f.tlsConfig)
	}
	f.addr = ln.Addr().String()
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		if f.silent {
			time.Sleep(2 * time.Second)
			return
		}
		f.serve(conn)
	}()
}

func (f *fakeServer) record(cmd string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, cmd)
}

func (f *fakeServer) serve(conn net.Conn) {
	r, w := bufio.NewReader(conn), bufio.NewWriter(conn)
	reply := func(s string) {
		_, _ = w.WriteString(s + "\r\n")
		_ = w.Flush()
	}
	reply("220 fake ESMTP")
	secure := f.implicit
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimRight(line, "\r\n")
		f.record(cmd)
		verb := strings.ToUpper(strings.SplitN(cmd, " ", 2)[0])
		switch {
		case verb == "EHLO":
			reply("250-fake")
			if f.starttls && !secure {
				reply("250-STARTTLS")
			}
			reply("250 AUTH PLAIN")
		case verb == "STARTTLS":
			reply("220 go ahead")
			tc := tls.Server(conn, f.tlsConfig)
			if tc.Handshake() != nil {
				return
			}
			conn, secure = tc, true
			f.mu.Lock()
			f.tlsUsed = true
			f.mu.Unlock()
			r, w = bufio.NewReader(conn), bufio.NewWriter(conn)
		case verb == "AUTH":
			f.mu.Lock()
			f.auth = cmd
			f.mu.Unlock()
			reply("235 ok")
		case verb == "MAIL" && f.reject:
			reply("550 no")
		case verb == "MAIL", verb == "RCPT":
			reply("250 ok")
		case verb == "DATA":
			reply("354 go")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil || l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			f.mu.Lock()
			f.data = b.String()
			f.mu.Unlock()
			reply("250 queued")
		case verb == "QUIT":
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

func sender(t *testing.T, f *fakeServer, mode string, user, pass string, pool *x509.CertPool) *mailer.SMTP {
	t.Helper()
	host, port, err := net.SplitHostPort(f.addr)
	require.NoError(t, err)
	n, err := strconv.Atoi(port)
	require.NoError(t, err)
	var cfg *tls.Config
	if pool != nil {
		cfg = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return mailer.NewSMTP(mailer.SMTPOptions{
		Host: host, Port: n, Username: user, Password: pass, TLS: mode, Timeout: time.Second, TLSConfig: cfg,
		From: netmail.Address{Name: "داو Dhow", Address: "dhow@example.com"},
	})
}

var message = mailer.Message{
	To: "ali@future.dev", Subject: "منى دعتك لـ Future Solution", Body: "Open https://dhow.example.com/invite#token\nIt works once.",
}

func decodedBody(t *testing.T, data string) string {
	t.Helper()
	_, body, ok := strings.Cut(data, "\r\n\r\n")
	require.True(t, ok)
	raw, err := readAll(quotedprintable.NewReader(strings.NewReader(body)))
	require.NoError(t, err)
	return raw
}

func readAll(r io.Reader) (string, error) {
	var b strings.Builder
	buf := make([]byte, 512)
	for {
		n, err := r.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			if errors.Is(err, io.EOF) {
				return b.String(), nil
			}
			return b.String(), err
		}
	}
}

func TestSendWithoutTLSToARelay(t *testing.T) {
	f := &fakeServer{}
	f.start(t)
	require.NoError(t, sender(t, f, mailer.TLSNone, "", "", nil).Send(context.Background(), message))
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Contains(t, f.commands, "MAIL FROM:<dhow@example.com>")
	assert.Contains(t, f.commands, "RCPT TO:<ali@future.dev>")
	assert.Contains(t, f.data, "Subject: =?utf-8?q?")
	assert.Contains(t, f.data, "Content-Type: text/plain; charset=utf-8")
	assert.Contains(t, f.data, "Auto-Submitted: auto-generated")
	assert.Contains(t, decodedBody(t, f.data), "https://dhow.example.com/invite#token\r\nIt works once.")
	assert.Empty(t, f.auth)
}

func TestSendWithStartTLSAndLogin(t *testing.T) {
	cfg, pool := selfSigned(t)
	f := &fakeServer{starttls: true, tlsConfig: cfg}
	f.start(t)
	require.NoError(t, sender(t, f, mailer.TLSStartTLS, "dhow", "relay-credential", pool).Send(context.Background(), message))
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.True(t, f.tlsUsed)
	want := "AUTH PLAIN " + base64.StdEncoding.EncodeToString([]byte("\x00dhow\x00relay-credential"))
	assert.Equal(t, want, f.auth)
}

func TestSendWithImplicitTLS(t *testing.T) {
	cfg, pool := selfSigned(t)
	f := &fakeServer{implicit: true, tlsConfig: cfg}
	f.start(t)
	require.NoError(t, sender(t, f, mailer.TLSImplicit, "dhow", "relay-credential", pool).Send(context.Background(), message))
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.NotEmpty(t, f.auth)
	assert.NotEmpty(t, f.data)
}

func TestStartTLSIsRequiredBeforeTheCredentialsLeave(t *testing.T) {
	f := &fakeServer{}
	f.start(t)
	err := sender(t, f, mailer.TLSStartTLS, "dhow", "relay-credential", nil).Send(context.Background(), message)
	require.ErrorIs(t, err, mailer.ErrNoStartTLS)
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Empty(t, f.auth)
	for _, c := range f.commands {
		assert.NotContains(t, c, "MAIL")
	}
}

func TestAnUntrustedCertificateIsRefused(t *testing.T) {
	cfg, _ := selfSigned(t)
	f := &fakeServer{starttls: true, tlsConfig: cfg}
	f.start(t)
	err := sender(t, f, mailer.TLSStartTLS, "dhow", "relay-credential", nil).Send(context.Background(), message)
	require.Error(t, err)
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Empty(t, f.auth)
}

func TestASilentServerTimesOut(t *testing.T) {
	f := &fakeServer{silent: true}
	f.start(t)
	start := time.Now()
	err := sender(t, f, mailer.TLSNone, "", "", nil).Send(context.Background(), message)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 1900*time.Millisecond)
}

func TestARejectedSenderIsAnError(t *testing.T) {
	f := &fakeServer{reject: true}
	f.start(t)
	require.Error(t, sender(t, f, mailer.TLSNone, "", "", nil).Send(context.Background(), message))
}

func TestHeaderInjectionIsRefused(t *testing.T) {
	s := mailer.NewSMTP(mailer.SMTPOptions{
		Host: "127.0.0.1", Port: 1, TLS: mailer.TLSNone, Timeout: time.Second,
		From: netmail.Address{Address: "dhow@example.com"},
	})
	for _, m := range []mailer.Message{
		{To: "a@example.com", Subject: "hi\r\nBcc: victim@example.com", Body: "x"},
		{To: "a@example.com\r\nBcc: victim@example.com", Subject: "hi", Body: "x"},
	} {
		assert.ErrorIs(t, s.Send(context.Background(), m), mailer.ErrHeaderInjection)
	}
}

func TestDisabledSenderNeverSends(t *testing.T) {
	var d mailer.Disabled
	assert.False(t, d.Enabled())
	assert.ErrorIs(t, d.Send(context.Background(), message), mailer.ErrDisabled)
}
