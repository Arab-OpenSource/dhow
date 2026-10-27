package main

import (
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // RFC 6238 TOTP uses HMAC-SHA-1, exactly like the server and every authenticator app.
	"encoding/base32"
	"encoding/binary"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	period = 30
	digits = 1000000
	usage  = "usage: totp BASE32_SECRET [STEP_OFFSET]\n"
)

func code(secret []byte, step uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], step)
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	text := strconv.FormatUint(uint64(value%digits), 10)
	return strings.Repeat("0", 6-len(text)) + text
}

func run(args []string) int {
	if len(args) < 1 || len(args) > 2 {
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(args[0]))
	if err != nil {
		_, _ = os.Stderr.WriteString("the secret is not base32\n")
		return 2
	}
	offset := int64(0)
	if len(args) == 2 {
		if offset, err = strconv.ParseInt(args[1], 10, 64); err != nil {
			_, _ = os.Stderr.WriteString(usage)
			return 2
		}
	}
	step := time.Now().Unix()/period + offset
	if step < 0 {
		_, _ = os.Stderr.WriteString(usage)
		return 2
	}
	_, _ = os.Stdout.WriteString(code(secret, uint64(step)) + "\n")
	return 0
}

func main() {
	os.Exit(run(os.Args[1:]))
}
