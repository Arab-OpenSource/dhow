package e2e_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestListFiltersOverHTTP(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug).String()
	admin := env.SeedUser(env.OrganizationID(d.OrgSlug), "admin@example.com", "a long password for admin", "admin")
	_, login := env.Login("admin@example.com", "a long password for admin")
	require.Equal(t, http.StatusOK, login.Status)
	actor := admin.String()
	future := url.QueryEscape(time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	past := url.QueryEscape(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"a category", "?Category=system", []string{"system.instance.setupCompleted"}},
		{"an action", "?Action=auth.login", []string{"auth.login", "auth.login"}},
		{"an actor", "?ActorUserId=" + actor, []string{"auth.login"}},
		{"a window that holds everything", "?From=" + past + "&To=" + future, []string{"auth.login", "auth.login", "system.instance.setupCompleted"}},
		{"a window in the future", "?From=" + future, []string{}},
		{"an unknown action", "?Action=nothing.here", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, res := listEvents(t, client, orgID, tt.query)
			got := make([]string, 0, len(events))
			for _, e := range events {
				got = append(got, e.Action)
			}
			assert.Equal(t, tt.want, got)
			assert.Equal(t, len(tt.want), res.Meta.Total)
		})
	}
}

func TestListRejectsBadFilters(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug).String()
	later := url.QueryEscape(time.Now().UTC().Format(time.RFC3339))
	earlier := url.QueryEscape(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	tests := []struct {
		name   string
		query  string
		status int
		code   string
	}{
		{"an unknown category", "?Category=billing", http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"from after to", "?From=" + later + "&To=" + earlier, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"an actor that is not a uuid", "?ActorUserId=alice", http.StatusBadRequest, "BAD_REQUEST"},
		{"a time that is not a time", "?From=yesterday", http.StatusBadRequest, "BAD_REQUEST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := client.Get("/organizations/" + orgID + "/audit-events" + tt.query)
			assert.Equal(t, tt.status, res.Status, string(res.Raw))
			require.NotNil(t, res.Error)
			assert.Equal(t, tt.code, res.Error.Code)
		})
	}
}

func TestGetOneEvent(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug)
	events, _ := listEvents(t, client, orgID.String(), "")
	require.NotEmpty(t, events)

	res := client.Get("/organizations/" + orgID.String() + "/audit-events/" + events[0].Id)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var got event
	res.Decode(t, &got)
	assert.Equal(t, events[0], got)

	otherOrg := env.SeedOrganization("Other Org", "other-org")
	env.SeedUser(otherOrg, "other-owner@example.com", "a long password for other", "owner")
	other, login := env.Login("other-owner@example.com", "a long password for other")
	require.Equal(t, http.StatusOK, login.Status)
	theirs, _ := listEvents(t, other, otherOrg.String(), "")
	require.NotEmpty(t, theirs)
	missing := client.Get("/organizations/" + orgID.String() + "/audit-events/" + theirs[0].Id)
	assert.Equal(t, http.StatusNotFound, missing.Status, "an event of another organization is never read through this one")
	unknown := client.Get("/organizations/" + orgID.String() + "/audit-events/00000000-0000-7000-8000-0000000000aa")
	assert.Equal(t, http.StatusNotFound, unknown.Status)
}
