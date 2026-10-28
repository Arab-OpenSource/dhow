package auth

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

const (
	FactorPassword     = "password"
	FactorTotp         = "totp"
	FactorRecoveryCode = "recovery_code"
	FactorPasskey      = "passkey"
)

type Factor struct {
	Method      string
	Password    string
	Code        string
	ChallengeID uuid.UUID
	Credential  json.RawMessage
}

type FactorConfirmer interface {
	ConfirmFactor(ctx context.Context, p Principal, f Factor) error
}
