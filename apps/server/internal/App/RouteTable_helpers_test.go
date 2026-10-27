package app_test

import (
	"net/http"

	"github.com/google/uuid"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func mustUUID(v string) uuid.UUID { return uuid.MustParse(v) }

func sessionIDOf(c *apptest.Client) string {
	var me struct{ SessionId string }
	res := c.Do(apptest.Request{Method: http.MethodGet, Path: "/me"})
	res.Decode(c.Env.T, &me)
	return me.SessionId
}
