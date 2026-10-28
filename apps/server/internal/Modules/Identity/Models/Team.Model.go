package models

import (
	"time"

	"github.com/google/uuid"
)

type RoleBinding struct {
	ID        uuid.UUID
	Scope     string
	Role      string
	CreatedAt time.Time
}

type Team struct {
	ID          uuid.UUID
	Name        string
	Description string
	MemberCount int
	Bindings    []RoleBinding
	CreatedAt   time.Time
}

type TeamMember struct {
	UserID  uuid.UUID
	Name    string
	Email   string
	Role    string
	AddedAt time.Time
}

type TeamDetail struct {
	Team
	Members []TeamMember
}

type TeamInput struct {
	Name        string
	Description string
	Role        string
}

type TeamChange struct {
	Team    TeamDetail
	Revoked int64
}
