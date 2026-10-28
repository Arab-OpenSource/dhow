package mailer

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	netmail "net/mail"
	"strings"
	"time"
)

var ErrHeaderInjection = errors.New("a mail header value contains a line break")

type Message struct {
	To      string
	Subject string
	Body    string
}

func headerSafe(values ...string) error {
	for _, v := range values {
		if strings.ContainsAny(v, "\r\n") {
			return ErrHeaderInjection
		}
	}
	return nil
}

func build(from netmail.Address, m Message, now time.Time) ([]byte, error) {
	if err := headerSafe(from.Name, from.Address, m.To, m.Subject); err != nil {
		return nil, err
	}
	to, err := netmail.ParseAddress(m.To)
	if err != nil {
		return nil, fmt.Errorf("parse recipient: %w", err)
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("read random bytes: %w", err)
	}
	domain := from.Address[strings.LastIndex(from.Address, "@")+1:]
	var out bytes.Buffer
	headers := [][2]string{
		{"From", from.String()},
		{"To", to.String()},
		{"Subject", mime.QEncoding.Encode("utf-8", m.Subject)},
		{"Date", now.UTC().Format(time.RFC1123Z)},
		{"Message-ID", "<" + hex.EncodeToString(id) + "@" + domain + ">"},
		{"MIME-Version", "1.0"},
		{"Content-Type", "text/plain; charset=utf-8"},
		{"Content-Transfer-Encoding", "quoted-printable"},
		{"Auto-Submitted", "auto-generated"},
	}
	for _, h := range headers {
		out.WriteString(h[0] + ": " + h[1] + "\r\n")
	}
	out.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&out)
	if _, err := qp.Write([]byte(strings.ReplaceAll(m.Body, "\n", "\r\n"))); err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	if err := qp.Close(); err != nil {
		return nil, fmt.Errorf("encode body: %w", err)
	}
	out.WriteString("\r\n")
	return out.Bytes(), nil
}
