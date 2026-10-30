package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func overdue(w world, rule string) {
	past := w.env.Clock.Now().Add(-8 * 24 * time.Hour)
	w.env.SetSecondFactorRule(w.org, rule, past)
	w.env.SetJoinedAt(w.org, past)
}

func TestManagingAProjectNeedsTheSecondFactorTheRuleAsksFor(t *testing.T) {
	w := newWorld(t)
	project := w.env.SeedProject(w.org, w.ownerID)
	admin, adminID := w.member(t, "admin")
	developer, devID := w.member(t, "developer")
	w.env.SeedUserProjectBinding(w.org, project, devID, "admin")
	plain, _ := w.member(t, "maintainer")
	overdue(w, "admins")

	automatic := apptest.Request{Method: http.MethodPatch, Path: w.path("/" + project.String()), Body: map[string]any{"ForkPolicy": "automatic"}}
	rename := apptest.Request{Method: http.MethodPatch, Path: w.path("/" + project.String()), Body: map[string]any{"Name": "Renamed"}}
	requireError(t, admin.Do(automatic), http.StatusForbidden, "SECOND_FACTOR_REQUIRED")
	requireError(t, admin.Do(rename), http.StatusForbidden, "SECOND_FACTOR_REQUIRED")
	requireError(t, developer.Do(automatic), http.StatusForbidden, "SECOND_FACTOR_REQUIRED")
	requireError(t, create(admin, w, map[string]any{"Name": "New", "Slug": "new"}), http.StatusForbidden, "SECOND_FACTOR_REQUIRED")
	decode(t, admin.Get(w.path("/"+project.String())), http.StatusOK)
	decode(t, plain.Do(rename), http.StatusOK)

	w.env.SeedPasskey(adminID)
	assert.Equal(t, "automatic", decode(t, admin.Do(automatic), http.StatusOK).ForkPolicy)
	w.env.SeedPasskey(devID)
	decode(t, developer.Do(rename), http.StatusOK)
}

func TestTheEveryoneRuleCoversProjectManagementForEveryRole(t *testing.T) {
	w := newWorld(t)
	project := w.env.SeedProject(w.org, w.ownerID)
	maintainer, _ := w.member(t, "maintainer")
	overdue(w, "everyone")
	rename := apptest.Request{Method: http.MethodPatch, Path: w.path("/" + project.String()), Body: map[string]any{"Name": "Renamed"}}
	requireError(t, maintainer.Do(rename), http.StatusForbidden, "SECOND_FACTOR_REQUIRED")
	decode(t, maintainer.Get(w.path("/"+project.String())), http.StatusOK)
}
