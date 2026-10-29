package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	EmailDisabled = "disabled"
	EmailSent     = "sent"
	EmailFailed   = "failed"

	InvitationsOpen     = "open"
	InvitationsAccepted = "accepted"
)

type Invitation struct {
	ID            uuid.UUID
	Email         string
	Role          string
	InvitedByName string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	SentAt        time.Time
	SendCount     int
	EmailStatus   string
	Expired       bool
	Accepted      *InvitationAcceptance
}

type InvitationAcceptance struct {
	At       time.Time
	UserID   uuid.UUID
	UserName string
}

type InvitationIssued struct {
	Invitation Invitation
	Link       string
}

type InvitationPreview struct {
	OrganizationName string
	InviterName      string
	Email            string
	Role             string
	ExpiresAt        time.Time
	AccountExists    bool
}

type AcceptInput struct {
	Token    string
	Name     string
	Password string
	Locale   string
}

type InvitationAccepted struct {
	User         User
	Organization Organization
	Role         string
}
