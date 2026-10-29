package apptest

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type JobSeed struct {
	State      string
	Labels     []string
	Isolation  string
	FinishedAt *time.Time
	Deleted    bool
}

type RunnerSeed struct {
	State      string
	LastSeenAt *time.Time
	Labels     []string
	Isolation  []string
	Deleted    bool
}

type InvitationSeed struct {
	ExpiresAt time.Time
	Accepted  bool
	Revoked   bool
	Deleted   bool
}

func (e *Env) randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(e.T, err)
	return b
}

func jobPath(state string) []string {
	switch state {
	case "", "queued":
		return nil
	case "leased":
		return []string{"leased"}
	case "running":
		return []string{"leased", "running"}
	default:
		return []string{"leased", "running", state}
	}
}

func (e *Env) SeedJobWith(org, creator uuid.UUID, s JobSeed) uuid.UUID {
	e.T.Helper()
	if s.Isolation == "" {
		s.Isolation = "container"
	}
	if s.Labels == nil {
		s.Labels = []string{}
	}
	ctx := context.Background()
	var id uuid.UUID
	require.NoError(e.T, e.Pool.QueryRow(ctx,
		`INSERT INTO "Jobs" ("OrganizationId", "CreatedByUserId", "Name", "Spec", "SpecDigest", "Image", "Isolation", "Labels",
		 "QueueDeadline") VALUES ($1, $2, 'overview', '{}'::jsonb, $3, $4, $5, $6, now() + interval '1 hour') RETURNING "Id"`,
		org, creator, e.randomBytes(32), TestImage, s.Isolation, s.Labels).Scan(&id))
	for _, state := range jobPath(s.State) {
		var finished *time.Time
		if state == s.State {
			finished = s.FinishedAt
		}
		_, err := e.Pool.Exec(ctx, `UPDATE "Jobs" SET "State" = $2, "FinishedAt" = $3 WHERE "Id" = $1`, id, state, finished)
		require.NoError(e.T, err)
	}
	if s.Deleted {
		_, err := e.Pool.Exec(ctx, `UPDATE "Jobs" SET "IsDeleted" = true WHERE "Id" = $1`, id)
		require.NoError(e.T, err)
	}
	return id
}

func (e *Env) SeedRunnerWith(org, creator uuid.UUID, s RunnerSeed) uuid.UUID {
	e.T.Helper()
	if s.State == "" {
		s.State = "active"
	}
	if s.Isolation == nil {
		s.Isolation = []string{"container"}
	}
	if s.Labels == nil {
		s.Labels = []string{}
	}
	reason := map[string]string{"disabled": "manual"}[s.State]
	var revokedAt *time.Time
	if s.State == "revoked" {
		at := e.Clock.Now()
		revokedAt = &at
	}
	token := e.SeedRegistrationToken(org, creator)
	var id uuid.UUID
	require.NoError(e.T, e.Pool.QueryRow(context.Background(),
		`INSERT INTO "Runners" ("OrganizationId", "RegistrationTokenId", "Name", "Labels", "Os", "Arch", "Version", "IsolationLevels",
		 "Capacity", "SigningPublicKey", "EncryptionPublicKey", "State", "DisabledReason", "RevokedAt", "LastSeenAt", "IsDeleted")
		 VALUES ($1, $2, 'overview', $3, 'linux', 'amd64', '0.1.0', $4, 1, $5, $6, $7, $8, $9, $10, $11) RETURNING "Id"`,
		org, token, s.Labels, s.Isolation, e.randomBytes(32), e.randomBytes(1216), s.State, reason, revokedAt, s.LastSeenAt,
		s.Deleted).Scan(&id))
	return id
}

func (e *Env) SeedInvitationWith(org, inviter uuid.UUID, s InvitationSeed) uuid.UUID {
	e.T.Helper()
	var acceptedAt, revokedAt *time.Time
	var acceptedBy *uuid.UUID
	now := e.Clock.Now()
	if s.Accepted {
		acceptedAt, acceptedBy = &now, &inviter
	}
	if s.Revoked {
		revokedAt = &now
	}
	var id uuid.UUID
	require.NoError(e.T, e.Pool.QueryRow(context.Background(),
		`INSERT INTO "Invitations" ("OrganizationId", "Email", "Role", "TokenHash", "InvitedByUserId", "ExpiresAt", "AcceptedAt",
		 "AcceptedByUserId", "RevokedAt", "IsDeleted")
		 VALUES ($1, $2, 'viewer', $3, $4, $5, $6, $7, $8, $9) RETURNING "Id"`,
		org, uuid.NewString()+"@example.com", e.randomBytes(32), inviter, s.ExpiresAt, acceptedAt, acceptedBy, revokedAt,
		s.Deleted).Scan(&id))
	return id
}
