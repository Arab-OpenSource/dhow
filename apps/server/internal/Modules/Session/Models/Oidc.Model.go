package models

import (
	"time"

	"github.com/google/uuid"
)

const MethodOidc = "oidc"

type OidcProvider struct {
	ID   string
	Name string
	Kind string
}

type OidcAuthorization struct {
	URL         string
	ExpiresAt   time.Time
	CookieValue string
}

type OidcCallback struct {
	Provider string
	Code     string
	State    string
}

type LinkedIdentity struct {
	ID           uuid.UUID
	Provider     string
	ProviderName string
	Email        string
	CreatedAt    time.Time
	LastUsedAt   *time.Time
}

type IdentityChange struct {
	Identity LinkedIdentity
	Revoked  int64
}
