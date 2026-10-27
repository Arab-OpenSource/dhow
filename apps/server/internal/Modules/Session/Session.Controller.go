package session

import (
	"context"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Models"
)

const (
	messageLogin           = "session.login.succeeded"
	messageLogout          = "session.logout.succeeded"
	messageMe              = "session.me.loaded"
	messageSessions        = "session.list.loaded"
	messageRevoked         = "session.revoke.succeeded"
	messageRevokedOthers   = "session.revokeOthers.succeeded"
	messagePasswordChanged = "session.password.changed"
)

type Controller struct {
	service *Service
}

var _ gen.StrictServerInterface = (*Controller)(nil)

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (c *Controller) Login(ctx context.Context, req gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.Login(ctx, models.LoginInput{Email: req.Body.Email, Password: req.Body.Password})
	if err != nil {
		return nil, err
	}
	data := gen.LoginResult{User: userToGen(res.User), CsrfToken: res.CSRFToken, ExpiresAt: res.ExpiresAt}
	return gen.Login200JSONResponse{
		Body:    gen.LoginResponse{Success: true, Message: ptr(messageLogin), Data: &data},
		Headers: gen.Login200ResponseHeaders{SetCookie: auth.SessionCookie(config.SessionCookieName, res.CookieValue, config.SessionAbsoluteTimeout)},
	}, nil
}

func (c *Controller) Logout(ctx context.Context, _ gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	if err := c.service.Logout(ctx); err != nil {
		return nil, err
	}
	return gen.Logout200JSONResponse{
		Body:    gen.Envelope{Success: true, Message: ptr(messageLogout)},
		Headers: gen.Logout200ResponseHeaders{SetCookie: auth.ClearedSessionCookie(config.SessionCookieName)},
	}, nil
}

func (c *Controller) GetMe(ctx context.Context, _ gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	me, err := c.service.Me(ctx)
	if err != nil {
		return nil, err
	}
	memberships := make([]gen.MembershipView, 0, len(me.Memberships))
	for _, m := range me.Memberships {
		memberships = append(memberships, gen.MembershipView{
			OrganizationId: m.OrganizationID, OrganizationName: m.OrganizationName,
			OrganizationSlug: m.OrganizationSlug, Role: gen.Role(m.Role),
		})
	}
	data := gen.Me{User: userToGen(me.User), Memberships: memberships, CsrfToken: me.CSRFToken, SessionId: me.SessionID, ExpiresAt: me.ExpiresAt}
	return gen.GetMe200JSONResponse{Success: true, Message: ptr(messageMe), Data: &data}, nil
}

func (c *Controller) ListSessions(ctx context.Context, _ gen.ListSessionsRequestObject) (gen.ListSessionsResponseObject, error) {
	list, err := c.service.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]gen.SessionView, 0, len(list))
	for _, s := range list {
		items = append(items, gen.SessionView{
			Id: s.ID, CreatedAt: s.CreatedAt, LastSeenAt: s.LastSeenAt, ExpiresAt: s.ExpiresAt,
			UserAgent: s.UserAgent, IpAddress: s.IPAddress, Current: s.Current,
		})
	}
	return gen.ListSessions200JSONResponse{Success: true, Message: ptr(messageSessions), Data: &items}, nil
}

func (c *Controller) RevokeSession(ctx context.Context, req gen.RevokeSessionRequestObject) (gen.RevokeSessionResponseObject, error) {
	if err := c.service.Revoke(ctx, req.Id); err != nil {
		return nil, err
	}
	return gen.RevokeSession200JSONResponse{Success: true, Message: ptr(messageRevoked)}, nil
}

func (c *Controller) RevokeOtherSessions(ctx context.Context, _ gen.RevokeOtherSessionsRequestObject) (gen.RevokeOtherSessionsResponseObject, error) {
	n, err := c.service.RevokeOthers(ctx)
	if err != nil {
		return nil, err
	}
	return gen.RevokeOtherSessions200JSONResponse{
		Success: true, Message: ptr(messageRevokedOthers), Data: &gen.RevokedCount{Revoked: int(n)},
	}, nil
}

func (c *Controller) ChangePassword(ctx context.Context, req gen.ChangePasswordRequestObject) (gen.ChangePasswordResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	n, err := c.service.ChangePassword(ctx, req.Body.CurrentPassword, req.Body.NewPassword)
	if err != nil {
		return nil, err
	}
	return gen.ChangePassword200JSONResponse{
		Success: true, Message: ptr(messagePasswordChanged), Data: &gen.RevokedCount{Revoked: int(n)},
	}, nil
}

func userToGen(u models.User) gen.User {
	return gen.User{Id: u.ID, Email: u.Email, Name: u.Name, Locale: gen.Locale(u.Locale)}
}

func ptr[T any](v T) *T { return &v }
