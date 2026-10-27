package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonVersion   = argon2.Version
	maxMemoryKiB   = 1 << 20
	maxIterations  = 16
	maxParallelism = 16
	maxKeyLength   = 128
	maxSaltLength  = 128
	minSaltLength  = 8
	minKeyLength   = 16
)

var ErrMalformedHash = errors.New("malformed password hash")

type Params struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

func DefaultParams() Params {
	return Params{MemoryKiB: 19 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}
}

type Hasher struct {
	params Params
	slots  chan struct{}
	dummy  string
}

func NewHasher(ctx context.Context, p Params, maxConcurrent int) (*Hasher, error) {
	if p.MemoryKiB < 8 || p.Iterations < 1 || p.Parallelism < 1 || p.SaltLength < minSaltLength || p.KeyLength < minKeyLength {
		return nil, errors.New("invalid argon2id parameters")
	}
	h := &Hasher{params: p, slots: make(chan struct{}, max(maxConcurrent, 1))}
	filler := make([]byte, 24)
	if _, err := rand.Read(filler); err != nil {
		return nil, fmt.Errorf("read random bytes: %w", err)
	}
	dummy, err := h.Hash(ctx, base64.RawStdEncoding.EncodeToString(filler))
	if err != nil {
		return nil, err
	}
	h.dummy = dummy
	return h, nil
}

func (h *Hasher) Params() Params { return h.params }

func (h *Hasher) acquire(ctx context.Context) (func(), error) {
	select {
	case h.slots <- struct{}{}:
		return func() { <-h.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	release, err := h.acquire(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.params.Iterations, h.params.MemoryKiB, h.params.Parallelism, h.params.KeyLength)
	return encode(h.params, salt, key), nil
}

func (h *Hasher) Verify(ctx context.Context, password, encoded string) (ok, needsRehash bool, err error) {
	p, salt, want, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	release, err := h.acquire(ctx)
	if err != nil {
		return false, false, err
	}
	defer release()
	got := argon2.IDKey([]byte(password), salt, p.Iterations, p.MemoryKiB, p.Parallelism, clampUint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false, nil
	}
	return true, h.NeedsRehash(encoded), nil
}

func (h *Hasher) VerifyDummy(ctx context.Context, password string) error {
	_, _, err := h.Verify(ctx, password, h.dummy)
	return err
}

func (h *Hasher) NeedsRehash(encoded string) bool {
	p, salt, key, err := decode(encoded)
	if err != nil {
		return true
	}
	return p.MemoryKiB != h.params.MemoryKiB || p.Iterations != h.params.Iterations ||
		p.Parallelism != h.params.Parallelism || clampUint32(len(salt)) != h.params.SaltLength ||
		clampUint32(len(key)) != h.params.KeyLength
}

func encode(p Params, salt, key []byte) string {
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonVersion, p.MemoryKiB, p.Iterations, p.Parallelism, b64.EncodeToString(salt), b64.EncodeToString(key))
}

func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argonVersion {
		return Params{}, nil, nil, ErrMalformedHash
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Iterations, &p.Parallelism); err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	b64 := base64.RawStdEncoding.Strict()
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, ErrMalformedHash
	}
	p.SaltLength, p.KeyLength = clampUint32(len(salt)), clampUint32(len(key))
	if p.MemoryKiB < 8 || p.MemoryKiB > maxMemoryKiB || p.Iterations < 1 || p.Iterations > maxIterations ||
		p.Parallelism < 1 || p.Parallelism > maxParallelism || p.SaltLength < minSaltLength || p.SaltLength > maxSaltLength ||
		p.KeyLength < minKeyLength || p.KeyLength > maxKeyLength {
		return Params{}, nil, nil, ErrMalformedHash
	}
	return p, salt, key, nil
}

func clampUint32(n int) uint32 {
	if n < 0 {
		return 0
	}
	if n > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(n)
}
