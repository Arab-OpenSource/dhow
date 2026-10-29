package e2e_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	dbtest "github.com/Khaymah/dhow/apps/server/internal/Database/Testing"
)

func (o oidcEnv) link(t *testing.T, user uuid.UUID, provider, subject string) {
	t.Helper()
	_, err := o.env.Pool.Exec(t.Context(),
		`INSERT INTO "UserIdentities" ("UserId", "Provider", "Issuer", "Subject") VALUES ($1, $2, $3, $4)`,
		user, provider, o.idp.Issuer(), subject)
	require.NoError(t, err)
}

func TestTheEnvironmentListCountsTheAccountsItLeavesWithoutAProvider(t *testing.T) {
	pool := dbtest.NewPool(t)
	o := newOidcEnv(t, apptest.Options{Pool: pool})
	o.addCorp(t, nil)
	owner := o.userID(t, o.d.Email)
	dev := o.seedMember(t, "dev@example.com")
	gone := o.seedMember(t, "gone@example.com")
	o.link(t, owner, "corp", "owner-sub")
	o.link(t, dev, "corp", "dev-sub")
	o.link(t, gone, "corp", "gone-sub")
	o.link(t, dev, "staff", "dev-staff")
	_, err := pool.Exec(t.Context(), `UPDATE "Users" SET "IsDeleted" = true WHERE "Id" = $1`, gone)
	require.NoError(t, err)

	assert.Zero(t, readOidc(t, o.owner).IgnoredLinkedIdentities, "nothing is stranded while the dashboard list is in effect")

	locked := apptest.NewWithOptions(t, apptest.Options{Pool: pool, Clock: o.env.Clock, OidcTransport: o.idp.Transport(), Profile: envProvider(o.idp.Issuer() + "/staff")})
	v := readOidc(t, locked.SeedSession(owner))
	assert.True(t, v.Locked)
	assert.Equal(t, int64(1), v.IgnoredStored)
	assert.Equal(t, int64(2), v.IgnoredLinkedIdentities, "the two live accounts on corp, not the deleted user nor the env provider's own link")

	reused := apptest.NewWithOptions(t, apptest.Options{Pool: pool, Clock: o.env.Clock, OidcTransport: o.idp.Transport(), Profile: func(c *config.Config) {
		envProvider(o.idp.Issuer())(c)
		c.OIDCProviders[0].ID = "corp"
	}})
	again := readOidc(t, reused.SeedSession(owner))
	assert.True(t, again.Locked)
	assert.Zero(t, again.IgnoredLinkedIdentities, "an env provider with the same id keeps its accounts")
}
