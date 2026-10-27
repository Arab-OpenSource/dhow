package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	MethodPassword     = "password"
	MethodTotp         = "totp"
	MethodRecoveryCode = "recovery_code"
	MethodPasskey      = "passkey"
)

type MfaPending struct {
	Methods     []string
	ExpiresAt   time.Time
	CookieValue string
}

type MfaLoginInput struct {
	Method     string
	Code       string
	Credential json.RawMessage
}

type Factor struct {
	Method      string
	Password    string
	Code        string
	ChallengeID uuid.UUID
	Credential  json.RawMessage
}

type MfaStatus struct {
	Enabled                bool
	TotpEnabled            bool
	TotpEnabledAt          *time.Time
	RecoveryCodesRemaining int
	PasskeyCount           int
}

type TotpEnrollment struct {
	Secret    string
	URI       string
	ExpiresAt time.Time
}

type MfaChange struct {
	Revoked       int64
	RecoveryCodes []string
}

type PasskeyChallenge struct {
	ChallengeID uuid.UUID
	Options     json.RawMessage
	ExpiresAt   time.Time
}

type Passkey struct {
	ID         uuid.UUID
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	Transports []string
	Synced     bool
}

type AddPasskeyInput struct {
	ChallengeID uuid.UUID
	Credential  json.RawMessage
	Name        string
	Factor      Factor
}

type AddPasskeyResult struct {
	Passkey Passkey
	Change  MfaChange
}
