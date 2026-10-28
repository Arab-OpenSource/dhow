package e2e_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func signInMethodCount(t *testing.T, env *apptest.Env) int {
	t.Helper()
	var n int
	require.NoError(t, env.Pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM "UserIdentities" WHERE NOT "IsDeleted") + (SELECT count(*) FROM "Passkeys" WHERE NOT "IsDeleted")`).Scan(&n))
	return n
}

func TestRemovingAPasskeyAndUnlinkingAtOnceKeepsOneSignInMethod(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{})
	enrolled := owner.EnableTotp(apptest.PasswordFactor(d.Password))
	codes := enrolled.RecoveryCodes
	require.GreaterOrEqual(t, len(codes), 4)
	added := owner.AddPasskey(authenticator(t), "Key", apptest.CodeFactor("recovery_code", codes[0]), apptest.Override{})
	require.Equal(t, http.StatusOK, added.Status, string(added.Raw))
	linked := owner.LinkOidc(idp, apptest.CodeFactor("recovery_code", codes[1]), sara())
	require.Equal(t, http.StatusOK, linked.Status, string(linked.Raw))
	var identity struct{ Identity struct{ Id string } }
	linked.Decode(t, &identity)
	var passkeys []passkeyView
	owner.Get("/me/passkeys").Decode(t, &passkeys)
	require.Len(t, passkeys, 1)
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Users" SET "PasswordHash" = '' WHERE "Email" = lower($1)`, d.Email)
	require.NoError(t, err)
	require.Equal(t, 2, signInMethodCount(t, env))

	paths := []string{"/me/passkeys/" + passkeys[0].Id + "/remove", "/me/identities/" + identity.Identity.Id + "/remove"}
	results := make([]apptest.Response, len(paths))
	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Go(func() {
			results[i] = owner.Do(apptest.Request{
				Method: http.MethodPost, Path: path, Body: map[string]any{"CurrentFactor": apptest.CodeFactor("recovery_code", codes[2+i])},
			})
		})
	}
	wg.Wait()

	statuses := map[int]int{}
	for _, res := range results {
		statuses[res.Status]++
		if res.Status == http.StatusConflict {
			require.NotNil(t, res.Error)
			assert.Equal(t, "LAST_SIGN_IN_METHOD", res.Error.Code)
		}
	}
	assert.Equal(t, map[int]int{http.StatusOK: 1, http.StatusConflict: 1}, statuses, "exactly one removal wins")
	assert.Equal(t, 1, signInMethodCount(t, env), "the account keeps one way to sign in")
}

func TestTheLastPasskeyCannotBeRemoved(t *testing.T) {
	env, c, d, a := withPasskey(t)
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Users" SET "PasswordHash" = '' WHERE "Email" = lower($1)`, d.Email)
	require.NoError(t, err)
	var list []passkeyView
	c.Get("/me/passkeys").Decode(t, &list)
	require.Len(t, list, 1)
	res := c.Do(apptest.Request{
		Method: http.MethodPost, Path: "/me/passkeys/" + list[0].Id + "/remove", Body: map[string]any{"CurrentFactor": c.PasskeyFactor(a)},
	})
	assert.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
	require.NotNil(t, res.Error)
	assert.Equal(t, "LAST_SIGN_IN_METHOD", res.Error.Code)
	assert.Equal(t, 1, signInMethodCount(t, env))
}
