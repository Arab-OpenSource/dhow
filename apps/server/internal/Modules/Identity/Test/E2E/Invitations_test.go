package e2e_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

const newcomerPassword = "a brand new passphrase 2026"

func TestInviteAcceptAndSignIn(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)

	issued := owner.MustInvite(org, "  Ali@Future.Dev ", "developer")
	assert.Equal(t, "ali@future.dev", issued.Invitation.Email)
	assert.Equal(t, "disabled", issued.Invitation.EmailStatus, "no SMTP, so no email")
	assert.True(t, strings.HasPrefix(issued.Link, apptest.PublicOrigin+"/invite#"))
	assert.Len(t, issued.Token, 43)

	var stored []byte
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT "TokenHash" FROM "Invitations" WHERE "Id" = $1`, issued.Invitation.Id).Scan(&stored))
	assert.Len(t, stored, 32)
	assert.NotContains(t, string(stored), issued.Token, "only the hash is stored")

	preview := env.Preview(issued.Token)
	require.Equal(t, http.StatusOK, preview.Status, string(preview.Raw))
	var p struct {
		OrganizationName, InviterName, Email, Role string
		AccountExists                              bool
	}
	preview.Decode(t, &p)
	assert.Equal(t, "Dhow Co", p.OrganizationName)
	assert.Equal(t, "Mona Owner", p.InviterName)
	assert.Equal(t, "developer", p.Role)
	assert.False(t, p.AccountExists)

	weak := env.AcceptNew(issued.Token, "Ali Hassan", "short")
	assert.Equal(t, http.StatusUnprocessableEntity, weak.Status)
	accepted := env.AcceptNew(issued.Token, "Ali Hassan", newcomerPassword)
	require.Equal(t, http.StatusOK, accepted.Status, string(accepted.Raw))
	assert.Empty(t, accepted.Header.Values("Set-Cookie"), "accepting creates no session")

	ali, login := env.Login("ali@future.dev", newcomerPassword)
	require.Equal(t, http.StatusOK, login.Status)
	assert.Equal(t, "developer", ali.RoleIn(org))

	reused := env.AcceptNew(issued.Token, "Someone Else", newcomerPassword)
	assert.Equal(t, http.StatusGone, reused.Status, "a used invitation is gone")
	assert.Equal(t, "INVITATION_INVALID", reused.Error.Code)
	assert.Equal(t, http.StatusGone, env.Preview(issued.Token).Status)
	assert.Equal(t, 1, env.AuditCount("identity.invitation.created"))
	assert.Equal(t, 1, env.AuditCount("identity.invitation.accepted"))
	assert.NotContains(t, env.Logs.String(), issued.Token, "the token never reaches the logs")
	assert.NotContains(t, env.Logs.String(), newcomerPassword)
}

func TestInvitationsAreCappedByTheInvitersRole(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	admin, adminID := env.SignedIn(org, "admin@example.com", "admin")
	maintainer, _ := env.SignedIn(org, "maint@example.com", "maintainer")

	assert.Equal(t, http.StatusUnprocessableEntity, owner.Invite(org, "x@example.com", "owner").Response.Status, "owner is never invited")
	assert.Equal(t, http.StatusForbidden, maintainer.Invite(org, "x@example.com", "viewer").Response.Status, "no member.manage")
	assert.Equal(t, http.StatusUnprocessableEntity, admin.Invite(org, "not-an-email", "viewer").Response.Status)
	issued := admin.MustInvite(org, "x@example.com", "admin")
	pending := owner.Invite(org, "X@example.com", "viewer")
	assert.Equal(t, http.StatusConflict, pending.Response.Status)
	assert.Equal(t, "INVITATION_PENDING", pending.Response.Error.Code)
	member := owner.Invite(org, "maint@example.com", "viewer")
	assert.Equal(t, "ALREADY_MEMBER", member.Response.Error.Code)

	demoted := owner.ChangeRole(org, adminID.String(), "developer", nil)
	require.Equal(t, http.StatusOK, demoted.Status)
	assert.Equal(t, http.StatusGone, env.Preview(issued.Token).Status, "an inviter who lost the authority voids the link")
	assert.Equal(t, http.StatusGone, env.AcceptNew(issued.Token, "X", newcomerPassword).Status)
}

func TestExpiredRevokedAndResentInvitations(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)

	expired := owner.MustInvite(org, "late@example.com", "viewer")
	env.Clock.Advance(config.InvitationTTL + time.Minute)
	owner, _ = env.Login(d.Email, d.Password)
	assert.Equal(t, http.StatusGone, env.Preview(expired.Token).Status, "expired")
	assert.Equal(t, http.StatusGone, env.AcceptNew(expired.Token, "Late", newcomerPassword).Status)
	var list []apptest.Invitation
	owner.Get(apptest.OrgPath(org, "/invitations")).Decode(t, &list)
	require.Len(t, list, 1)
	assert.True(t, list[0].Expired)

	resent := owner.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(org, "/invitations/"+expired.Invitation.Id+"/resend")})
	require.Equal(t, http.StatusOK, resent.Status, string(resent.Raw))
	var again apptest.Issued
	resent.Decode(t, &again)
	_, fresh, _ := strings.Cut(again.Link, "#")
	assert.NotEqual(t, expired.Token, fresh)
	assert.Equal(t, 2, again.Invitation.SendCount)
	assert.False(t, again.Invitation.Expired)
	assert.Equal(t, http.StatusOK, env.Preview(fresh).Status, "the new link works")

	revoked := owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(org, "/invitations/"+expired.Invitation.Id)})
	require.Equal(t, http.StatusOK, revoked.Status, string(revoked.Raw))
	assert.Equal(t, http.StatusGone, env.Preview(fresh).Status, "revoked")
	assert.Equal(t, http.StatusGone, env.AcceptNew(fresh, "Late", newcomerPassword).Status)
	assert.Equal(t, http.StatusNotFound, owner.Do(apptest.Request{
		Method: http.MethodDelete,
		Path:   apptest.OrgPath(org, "/invitations/"+expired.Invitation.Id),
	}).Status, "revoked twice")
	for _, action := range []string{"identity.invitation.resent", "identity.invitation.revoked"} {
		assert.Equal(t, 1, env.AuditCount(action), action)
	}
	for _, bogus := range []string{"", "AAAA", strings.Repeat("A", 43), expired.Token + "A"} {
		assert.Equal(t, http.StatusGone, env.Preview(bogus).Status, bogus)
	}
}

func TestExistingUserAcceptsSignedIn(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	otherOrg := env.SeedOrganization("Other", "other")
	bob, _ := env.SignedIn(otherOrg, "bob@example.com", "owner")
	eve, _ := env.SignedIn(otherOrg, "eve@example.com", "viewer")

	issued := owner.MustInvite(org, "bob@example.com", "maintainer")
	var p struct{ AccountExists bool }
	env.Preview(issued.Token).Decode(t, &p)
	assert.True(t, p.AccountExists)
	exists := env.AcceptNew(issued.Token, "Bob Again", newcomerPassword)
	assert.Equal(t, http.StatusConflict, exists.Status)
	assert.Equal(t, "INVITATION_ACCOUNT_EXISTS", exists.Error.Code)

	mismatch := eve.AcceptSignedIn(issued.Token)
	assert.Equal(t, http.StatusForbidden, mismatch.Status)
	assert.Equal(t, "INVITATION_EMAIL_MISMATCH", mismatch.Error.Code)
	assert.Empty(t, eve.RoleIn(org))

	joined := bob.AcceptSignedIn(issued.Token)
	require.Equal(t, http.StatusOK, joined.Status, string(joined.Raw))
	assert.Equal(t, "maintainer", bob.RoleIn(org))
	assert.Equal(t, "owner", bob.RoleIn(otherOrg))
	assert.Equal(t, http.StatusGone, bob.AcceptSignedIn(issued.Token).Status, "used")
}

func TestAMatchingEmailNeverJoinsWithoutTheToken(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	owner.MustInvite(org, "carol@example.com", "admin")
	env.SeedUserWithoutMembership("carol@example.com", "a long password for carol")
	carol, res := env.Login("carol@example.com", "a long password for carol")
	require.Equal(t, http.StatusOK, res.Status)
	assert.Empty(t, carol.Me().Memberships)
	assert.Equal(t, http.StatusNotFound, carol.Get(apptest.OrgPath(org, "")).Status)
}

func TestTwoParallelAcceptsExactlyOneWins(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	issued := owner.MustInvite(org, "race@example.com", "developer")

	const racers = 2
	statuses := make([]int, racers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			statuses[i] = env.AcceptNew(issued.Token, "Racer", newcomerPassword).Status
		}()
	}
	close(start)
	wg.Wait()
	wins := 0
	for _, s := range statuses {
		if s == http.StatusOK {
			wins++
		} else {
			assert.Contains(t, []int{http.StatusGone, http.StatusConflict}, s)
		}
	}
	assert.Equal(t, 1, wins, "%v", statuses)
	var users, memberships int
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT count(*) FROM "Users" WHERE "Email" = 'race@example.com'`).Scan(&users))
	require.NoError(t, env.Pool.QueryRow(t.Context(), `SELECT count(*) FROM "Memberships" m JOIN "Users" u ON u."Id" = m."UserId"
		WHERE u."Email" = 'race@example.com' AND NOT m."IsDeleted"`).Scan(&memberships))
	assert.Equal(t, 1, users)
	assert.Equal(t, 1, memberships)
}

func TestInvitationEmailIsFailSafe(t *testing.T) {
	mail := &apptest.FakeMailer{}
	env := apptest.NewWithOptions(t, apptest.Options{Mailer: mail})
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)

	sent := owner.MustInvite(org, "mailed@example.com", "viewer")
	assert.Equal(t, "sent", sent.Invitation.EmailStatus)
	msgs := mail.Messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "mailed@example.com", msgs[0].To)
	assert.Contains(t, msgs[0].Body, sent.Link)
	assert.Contains(t, msgs[0].Subject, "Dhow Co")
	assert.Contains(t, msgs[0].Body, "مشاهد", "the role is named in the inviter's language")

	mail.FailNext(true)
	failed := owner.MustInvite(org, "bounced@example.com", "viewer")
	assert.Equal(t, "failed", failed.Invitation.EmailStatus, "the invitation exists and says the email failed")
	assert.NotEmpty(t, failed.Link)
	assert.Equal(t, http.StatusOK, env.Preview(failed.Token).Status, "the link still works")
	assert.NotContains(t, env.Logs.String(), failed.Token)
}
