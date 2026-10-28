package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusActive   = "active"
	StatusExpired  = "expired"
	StatusRevoked  = "revoked"
	StatusInactive = "inactive"
)

type Owner struct {
	ID    uuid.UUID
	Name  string
	Email string
}

type Token struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	OrganizationName string
	Name             string
	Scopes           []string
	Status           string
	Owner            Owner
	CreatedAt        time.Time
	ExpiresAt        time.Time
	LastUsedAt       *time.Time
	LastUsedIP       string
	RevokedAt        *time.Time
}

type CreateInput struct {
	Name          string
	Scopes        []string
	ExpiresInDays int
}

type Created struct {
	Token Token
	Value string
}
