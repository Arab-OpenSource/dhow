package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID     uuid.UUID
	Email  string
	Name   string
	Locale string
}

type Membership struct {
	OrganizationID   uuid.UUID
	OrganizationName string
	OrganizationSlug string
	Role             string
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	User        User
	CSRFToken   string
	CookieValue string
	ExpiresAt   time.Time
	Pending     *MfaPending
}

type Me struct {
	User        User
	Memberships []Membership
	CSRFToken   string
	SessionID   uuid.UUID
	ExpiresAt   time.Time
}

type SessionInfo struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	UserAgent  string
	IPAddress  string
	Current    bool
}
