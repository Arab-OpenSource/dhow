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

type Organization struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
}

type Member struct {
	UserID   uuid.UUID
	Name     string
	Email    string
	Role     string
	JoinedAt time.Time
}

type SetupInput struct {
	Token            string
	OwnerName        string
	Email            string
	Password         string
	OrganizationName string
	OrganizationSlug string
	Locale           string
}

type SetupResult struct {
	User         User
	Organization Organization
}
