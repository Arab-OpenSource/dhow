package e2e_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

func TestAnOrganizationCannotQueueMoreThanTheCap(t *testing.T) {
	w := newWorld(t)
	seed := w.env.SeedJob(w.org, w.ownerID)
	_, err := w.env.Pool.Exec(context.Background(),
		`INSERT INTO "Jobs" ("OrganizationId", "CreatedByUserId", "Name", "Spec", "SpecDigest", "Image", "Isolation", "QueueDeadline")
		 SELECT "OrganizationId", "CreatedByUserId", "Name", "Spec", "SpecDigest", "Image", "Isolation", "QueueDeadline"
		 FROM "Jobs", generate_series(2, $2) WHERE "Id" = $1`, seed, config.JobMaxQueuedPerOrg)
	require.NoError(t, err)

	res := w.env.CreateJob(w.owner, w.org, "one too many", apptest.SimpleSpec(), nil)
	assert.Equal(t, http.StatusTooManyRequests, res.Status, string(res.Raw))
	require.NotNil(t, res.Error)
	assert.Equal(t, "JOB_QUEUE_FULL", res.Error.Code)

	require.Equal(t, http.StatusOK, w.cancel(t, seed).Status)
	res = w.env.CreateJob(w.owner, w.org, "fits again", apptest.SimpleSpec(), nil)
	assert.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	other := w.env.SeedOrganization("Other", "other")
	outsider := w.env.SeedSession(w.env.SeedUser(other, "o@example.com", "a long password for o", "owner"))
	res = w.env.CreateJob(outsider, other, "another organization", apptest.SimpleSpec(), nil)
	assert.Equal(t, http.StatusCreated, res.Status, "the cap is per organization")
}
