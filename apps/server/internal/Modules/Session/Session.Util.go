package session

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

const emailDigestLength = 32

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func emailDigest(email string) string {
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:])[:emailDigestLength]
}
