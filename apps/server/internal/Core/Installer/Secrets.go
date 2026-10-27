package installer

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // OpenPGP v4 key fingerprints are defined as SHA-1; this is not a security hash here.
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	SecretKeyBytes   = 32
	DBPasswordBytes  = 24
	scramIterations  = 4096
	scramSaltBytes   = 16
	pgpTagPublicKey  = 6
	pgpKeyVersion4   = 4
	pgpLenTwoOctets  = 192
	pgpLenFiveOctets = 255
	pgpLenPartial    = 224
)

var (
	ErrKeyFingerprint = errors.New("the downloaded signing key does not match the pinned fingerprint")
	ErrArmor          = errors.New("the signing key is not an ASCII-armored OpenPGP public key")
)

func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func ScramVerifier(password string) (string, error) {
	salt := make([]byte, scramSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return scramVerifierWithSalt(password, salt)
}

func scramVerifierWithSalt(password string, salt []byte) (string, error) {
	salted, err := pbkdf2.Key(sha256.New, password, salt, scramIterations, sha256.Size)
	if err != nil {
		return "", fmt.Errorf("derive password key: %w", err)
	}
	clientKey := hmacSHA256(salted, "Client Key")
	storedKey := sha256.Sum256(clientKey)
	serverKey := hmacSHA256(salted, "Server Key")
	enc := base64.StdEncoding
	return fmt.Sprintf("SCRAM-SHA-256$%d:%s$%s:%s", scramIterations, enc.EncodeToString(salt),
		enc.EncodeToString(storedKey[:]), enc.EncodeToString(serverKey)), nil
}

func hmacSHA256(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(msg))
	return h.Sum(nil)
}

func VerifyKeyFingerprint(armored []byte, want string) error {
	data, err := dearmor(string(armored))
	if err != nil {
		return err
	}
	prints, err := primaryFingerprints(data)
	if err != nil {
		return err
	}
	if len(prints) != 1 || !strings.EqualFold(prints[0], want) {
		return fmt.Errorf("%w (want %s, got %s)", ErrKeyFingerprint, want, strings.Join(prints, ", "))
	}
	return nil
}

func dearmor(s string) ([]byte, error) {
	begin := strings.Index(s, "-----BEGIN PGP PUBLIC KEY BLOCK-----")
	end := strings.Index(s, "-----END PGP PUBLIC KEY BLOCK-----")
	if begin < 0 || end < begin || strings.Count(s, "-----BEGIN PGP") != 1 {
		return nil, ErrArmor
	}
	lines := strings.Split(strings.ReplaceAll(s[begin:end], "\r", ""), "\n")[1:]
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			lines = lines[i+1:]
			break
		}
	}
	var b strings.Builder
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "=") {
			continue
		}
		b.WriteString(l)
	}
	data, err := base64.StdEncoding.DecodeString(b.String())
	if err != nil {
		return nil, ErrArmor
	}
	return data, nil
}

func primaryFingerprints(data []byte) ([]string, error) {
	var prints []string
	for len(data) > 0 {
		tag, body, rest, err := nextPacket(data)
		if err != nil {
			return nil, err
		}
		if tag == pgpTagPublicKey {
			if len(body) == 0 || body[0] != pgpKeyVersion4 {
				return nil, fmt.Errorf("%w: only v4 keys are supported", ErrArmor)
			}
			if len(body) > math.MaxUint16 {
				return nil, ErrArmor
			}
			size := uint16(len(body)) //nolint:gosec // Bounded by the check above.
			h := sha1.New()           //nolint:gosec // The v4 fingerprint is SHA-1 by definition (RFC 4880 12.2).
			h.Write(binary.BigEndian.AppendUint16([]byte{0x99}, size))
			h.Write(body)
			prints = append(prints, strings.ToUpper(hex.EncodeToString(h.Sum(nil))))
		}
		data = rest
	}
	return prints, nil
}

func nextPacket(data []byte) (byte, []byte, []byte, error) {
	if len(data) < 2 || data[0]&0x80 == 0 {
		return 0, nil, nil, ErrArmor
	}
	var tag byte
	var n, hdr int
	if data[0]&0x40 != 0 {
		tag = data[0] & 0x3f
		o1 := int(data[1])
		switch {
		case o1 < pgpLenTwoOctets:
			n, hdr = o1, 2
		case o1 < pgpLenPartial && len(data) >= 3:
			n, hdr = ((o1-pgpLenTwoOctets)<<8)+int(data[2])+pgpLenTwoOctets, 3
		case o1 == pgpLenFiveOctets && len(data) >= 6:
			n, hdr = int(binary.BigEndian.Uint32(data[2:6])), 6
		default:
			return 0, nil, nil, ErrArmor
		}
	} else {
		tag = (data[0] >> 2) & 0x0f
		switch data[0] & 0x03 {
		case 0:
			n, hdr = int(data[1]), 2
		case 1:
			if len(data) < 3 {
				return 0, nil, nil, ErrArmor
			}
			n, hdr = int(binary.BigEndian.Uint16(data[1:3])), 3
		case 2:
			if len(data) < 5 {
				return 0, nil, nil, ErrArmor
			}
			n, hdr = int(binary.BigEndian.Uint32(data[1:5])), 5
		default:
			return 0, nil, nil, ErrArmor
		}
	}
	if n < 0 || hdr+n > len(data) {
		return 0, nil, nil, ErrArmor
	}
	return tag, data[hdr : hdr+n], data[hdr+n:], nil
}
