package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ProviderGitHub  = "github"
	ProviderGitLab  = "gitlab"
	ProviderGeneric = "generic"

	TransportHTTPS = "https"
	TransportSSH   = "ssh"

	StateReady              = "ready"
	StateHostKeyUnconfirmed = "host_key_unconfirmed"

	DefaultBranch = "main"
)

type Repository struct {
	ID                         uuid.UUID
	OrganizationID             uuid.UUID
	ProjectID                  uuid.UUID
	Provider                   string
	CloneURL                   string
	Transport                  string
	FullName                   string
	DefaultBranch              string
	HookID                     string
	DeployKeyPublic            string
	PreviousDeployKeyPublic    string
	PreviousDeployKeyExpiresAt *time.Time
	HostKey                    string
	HostKeyFingerprint         string
	PendingHostKeyFingerprint  string
	State                      string
	LastDeliveryAt             *time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type LinkInput struct {
	CloneURL      string
	DefaultBranch *string
}

type Linked struct {
	Repository    Repository
	WebhookSecret string
	Created       bool
}
