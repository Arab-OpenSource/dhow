package auth

import (
	"fmt"
	"net/url"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func NewWebAuthn(publicURL *url.URL, displayName string, timeout time.Duration) (*webauthn.WebAuthn, error) {
	ceremony := webauthn.TimeoutConfig{Timeout: timeout, TimeoutUVD: timeout}
	w, err := webauthn.New(&webauthn.Config{
		RPID:                  publicURL.Hostname(),
		RPDisplayName:         displayName,
		RPOrigins:             []string{OriginOf(publicURL)},
		AttestationPreference: protocol.PreferNoAttestation,
		Timeouts:              webauthn.TimeoutsConfig{Login: ceremony, Registration: ceremony},
	})
	if err != nil {
		return nil, fmt.Errorf("configure webauthn for %s: %w", OriginOf(publicURL), err)
	}
	return w, nil
}
