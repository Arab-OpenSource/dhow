package apptest

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	mailer "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Mail"
)

type FakeMailer struct {
	mu       sync.Mutex
	fail     bool
	messages []mailer.Message
}

var ErrMailDown = errors.New("mail server is down")

func (f *FakeMailer) Enabled() bool { return true }

func (f *FakeMailer) Send(_ context.Context, m mailer.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return ErrMailDown
	}
	f.messages = append(f.messages, m)
	return nil
}

func (f *FakeMailer) FailNext(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = fail
}

func (f *FakeMailer) Messages() []mailer.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]mailer.Message(nil), f.messages...)
}

type Invitation struct {
	Id, Email, Role, EmailStatus string
	Expired                      bool
	SendCount                    int
}

type Issued struct {
	Invitation Invitation
	Link       string
	Token      string
	Response   Response
}

func OrgPath(org uuid.UUID, rest string) string { return "/organizations/" + org.String() + rest }

func (c *Client) Invite(org uuid.UUID, email, role string) Issued {
	c.Env.T.Helper()
	res := c.Do(Request{Method: http.MethodPost, Path: OrgPath(org, "/invitations"), Body: map[string]string{"Email": email, "Role": role}})
	out := Issued{Response: res}
	if res.Status == http.StatusCreated {
		res.Decode(c.Env.T, &out)
		_, out.Token, _ = strings.Cut(out.Link, "#")
	}
	return out
}

func (c *Client) MustInvite(org uuid.UUID, email, role string) Issued {
	c.Env.T.Helper()
	issued := c.Invite(org, email, role)
	require.Equal(c.Env.T, http.StatusCreated, issued.Response.Status, string(issued.Response.Raw))
	return issued
}

func (e *Env) AcceptNew(token, name, password string) Response {
	e.T.Helper()
	return e.Do(Request{
		Method: http.MethodPost, Path: "/invitations/accept", Headers: map[string]string{"Origin": PublicOrigin},
		Body: map[string]string{"Token": token, "Name": name, "Password": password},
	})
}

func (e *Env) Preview(token string) Response {
	e.T.Helper()
	return e.Do(Request{
		Method: http.MethodPost, Path: "/invitations/preview", Headers: map[string]string{"Origin": PublicOrigin},
		Body: map[string]string{"Token": token},
	})
}

func (c *Client) AcceptSignedIn(token string) Response {
	c.Env.T.Helper()
	return c.Do(Request{Method: http.MethodPost, Path: "/me/invitations/accept", Body: map[string]string{"Token": token}})
}

func (c *Client) ChangeRole(org uuid.UUID, user, role string, factor map[string]any) Response {
	c.Env.T.Helper()
	body := map[string]any{"Role": role}
	if factor != nil {
		body["CurrentFactor"] = factor
	}
	return c.Do(Request{Method: http.MethodPatch, Path: OrgPath(org, "/members/"+user), Body: body})
}

type MeView struct {
	User                   struct{ Id, Email string }
	Memberships            []struct{ OrganizationId, Role string }
	CanCreateOrganizations bool
	MfaReenrollRequired    bool
}

func (c *Client) Me() MeView {
	c.Env.T.Helper()
	var me MeView
	res := c.Get("/me")
	require.Equal(c.Env.T, http.StatusOK, res.Status, string(res.Raw))
	res.Decode(c.Env.T, &me)
	return me
}

func (c *Client) RoleIn(org uuid.UUID) string {
	c.Env.T.Helper()
	for _, m := range c.Me().Memberships {
		if m.OrganizationId == org.String() {
			return m.Role
		}
	}
	return ""
}

func (e *Env) SignedIn(org uuid.UUID, email, role string) (*Client, uuid.UUID) {
	e.T.Helper()
	password := "a long password for " + email
	id := e.SeedUser(org, email, password, role)
	c, res := e.Login(email, password)
	require.Equal(e.T, http.StatusOK, res.Status, string(res.Raw))
	return c, id
}

func (e *Env) SessionAlive(c *Client) bool {
	e.T.Helper()
	return c.Get("/me").Status == http.StatusOK
}
