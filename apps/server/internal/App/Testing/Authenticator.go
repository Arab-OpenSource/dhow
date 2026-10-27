package apptest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
)

const (
	flagUserPresent    = 0x01
	flagUserVerified   = 0x04
	flagBackupEligible = 0x08
	flagBackupState    = 0x10
	flagAttestedData   = 0x40
	coseKeyTypeEC2     = 2
	coseAlgES256       = -7
	coseCurveP256      = 1
)

var b64 = base64.RawURLEncoding

type Authenticator struct {
	key          *ecdsa.PrivateKey
	CredentialID []byte
	UserHandle   []byte
	SignCount    uint32
	Synced       bool
	SkipUV       bool
}

type ceremony struct {
	Challenge string
	RPID      string
	UserID    string
}

func field(m map[string]any, path ...string) string {
	var cur any = m
	for _, p := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = obj[p]
	}
	s, _ := cur.(string)
	return s
}

type Override struct {
	Origin string
	RPID   string
}

func NewAuthenticator() (*Authenticator, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		return nil, fmt.Errorf("read random bytes: %w", err)
	}
	return &Authenticator{key: key, CredentialID: id}, nil
}

func (a *Authenticator) flags() byte {
	f := byte(flagUserPresent)
	if !a.SkipUV {
		f |= flagUserVerified
	}
	if a.Synced {
		f |= flagBackupEligible | flagBackupState
	}
	return f
}

func (a *Authenticator) coseKey() ([]byte, error) {
	pub, err := a.key.PublicKey.ECDH()
	if err != nil {
		return nil, fmt.Errorf("export key: %w", err)
	}
	raw := pub.Bytes()
	return webauthncbor.Marshal(webauthncose.EC2PublicKeyData{
		PublicKeyData: webauthncose.PublicKeyData{KeyType: coseKeyTypeEC2, Algorithm: coseAlgES256},
		Curve:         coseCurveP256, XCoord: raw[1:33], YCoord: raw[33:65],
	})
}

func clientData(kind, challenge, origin string) []byte {
	raw, _ := json.Marshal(map[string]any{"type": kind, "challenge": challenge, "origin": origin, "crossOrigin": false})
	return raw
}

func authData(rpID string, flags byte, count uint32, attested []byte) []byte {
	hash := sha256.Sum256([]byte(rpID))
	out := append([]byte{}, hash[:]...)
	out = append(out, flags)
	out = binary.BigEndian.AppendUint32(out, count)
	return append(out, attested...)
}

func parse(options json.RawMessage, origin string, o Override) (ceremony, string, error) {
	var raw map[string]any
	if err := json.Unmarshal(options, &raw); err != nil {
		return ceremony{}, "", fmt.Errorf("decode options: %w", err)
	}
	c := ceremony{
		Challenge: field(raw, "publicKey", "challenge"), RPID: field(raw, "publicKey", "rpId"),
		UserID: field(raw, "publicKey", "user", "id"),
	}
	if c.RPID == "" {
		c.RPID = field(raw, "publicKey", "rp", "id")
	}
	if o.RPID != "" {
		c.RPID = o.RPID
	}
	if o.Origin != "" {
		origin = o.Origin
	}
	return c, origin, nil
}

func (a *Authenticator) Register(options json.RawMessage, origin string, o Override) (json.RawMessage, error) {
	c, origin, err := parse(options, origin, o)
	if err != nil {
		return nil, err
	}
	rpID := c.RPID
	if a.UserHandle, err = b64.DecodeString(c.UserID); err != nil {
		return nil, fmt.Errorf("decode user handle: %w", err)
	}
	key, err := a.coseKey()
	if err != nil {
		return nil, err
	}
	attested := make([]byte, 16)
	attested = binary.BigEndian.AppendUint16(attested, uint16(len(a.CredentialID))) //nolint:gosec // 32 bytes.
	attested = append(append(attested, a.CredentialID...), key...)
	object, err := webauthncbor.Marshal(map[string]any{
		"fmt": "none", "attStmt": map[string]any{}, "authData": authData(rpID, a.flags()|flagAttestedData, a.SignCount, attested),
	})
	if err != nil {
		return nil, fmt.Errorf("encode attestation: %w", err)
	}
	return json.Marshal(map[string]any{
		"id": b64.EncodeToString(a.CredentialID), "rawId": b64.EncodeToString(a.CredentialID), "type": "public-key",
		"authenticatorAttachment": "platform", "clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"clientDataJSON":    b64.EncodeToString(clientData("webauthn.create", c.Challenge, origin)),
			"attestationObject": b64.EncodeToString(object), "transports": []string{"internal", "hybrid"},
		},
	})
}

func (a *Authenticator) Assert(options json.RawMessage, origin string, o Override) (json.RawMessage, error) {
	c, origin, err := parse(options, origin, o)
	if err != nil {
		return nil, err
	}
	data := authData(c.RPID, a.flags(), a.SignCount, nil)
	client := clientData("webauthn.get", c.Challenge, origin)
	clientHash := sha256.Sum256(client)
	digest := sha256.Sum256(append(append([]byte{}, data...), clientHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	return json.Marshal(map[string]any{
		"id": b64.EncodeToString(a.CredentialID), "rawId": b64.EncodeToString(a.CredentialID), "type": "public-key",
		"authenticatorAttachment": "platform", "clientExtensionResults": map[string]any{},
		"response": map[string]any{
			"clientDataJSON": b64.EncodeToString(client), "authenticatorData": b64.EncodeToString(data),
			"signature": b64.EncodeToString(sig), "userHandle": b64.EncodeToString(a.UserHandle),
		},
	})
}
