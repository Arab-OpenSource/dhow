package jobs

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrSecretsUnavailable = errors.New("no secret store is configured")

type SecretRequest struct {
	OrganizationID uuid.UUID
	JobID          uuid.UUID
	AttemptID      uuid.UUID
	Names          []string
}

type SecretResolver interface {
	Accepts(ctx context.Context, organizationID uuid.UUID, names []string) error
	Resolve(ctx context.Context, req SecretRequest) (map[string][]byte, error)
}

type NoSecrets struct{}

func (NoSecrets) Accepts(context.Context, uuid.UUID, []string) error { return ErrSecretsUnavailable }

func (NoSecrets) Resolve(context.Context, SecretRequest) (map[string][]byte, error) {
	return nil, ErrSecretsUnavailable
}
