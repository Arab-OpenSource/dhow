package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Gen"
	models "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Models"
)

const (
	messageMfaRequired   = "session.mfa.required"
	messageMfaKeyOptions = "session.mfa.passkeyOptions"
	messageMfaStatus     = "mfa.status.loaded"
	messageTotpStarted   = "mfa.totp.started"
	messageTotpEnabled   = "mfa.totp.enabled"
	messageTotpDisabled  = "mfa.totp.disabled"
	messageRecoveryNew   = "mfa.recovery.regenerated"
	messageKeysListed    = "mfa.passkeys.listed"
	messageKeyOptions    = "mfa.passkey.options"
	messageKeyAdded      = "mfa.passkey.added"
	messageKeyRenamed    = "mfa.passkey.renamed"
	messageKeyRemoved    = "mfa.passkey.removed"
)

func pendingToGen(p *models.MfaPending) *gen.MfaPending {
	methods := make([]gen.MfaMethod, 0, len(p.Methods))
	for _, m := range p.Methods {
		methods = append(methods, gen.MfaMethod(m))
	}
	return &gen.MfaPending{Methods: methods, ExpiresAt: p.ExpiresAt}
}

func factorOf(f gen.CurrentFactor) models.Factor {
	out := models.Factor{Method: string(f.Method)}
	if f.Password != nil {
		out.Password = *f.Password
	}
	if f.Code != nil {
		out.Code = *f.Code
	}
	if f.ChallengeId != nil {
		out.ChallengeID = *f.ChallengeId
	}
	if f.Credential != nil {
		out.Credential = *f.Credential
	}
	return out
}

func changeToGen(c models.MfaChange) *gen.MfaChange {
	out := &gen.MfaChange{Revoked: int(c.Revoked)}
	if c.RecoveryCodes != nil {
		codes := c.RecoveryCodes
		out.RecoveryCodes = &codes
	}
	return out
}

func passkeyToGen(p models.Passkey) gen.Passkey {
	return gen.Passkey{Id: p.ID, Name: p.Name, CreatedAt: p.CreatedAt, LastUsedAt: p.LastUsedAt, Transports: p.Transports, Synced: p.Synced}
}

func challengeToGen(c models.PasskeyChallenge) *gen.PasskeyChallenge {
	return &gen.PasskeyChallenge{ChallengeId: c.ChallengeID, Options: c.Options, ExpiresAt: c.ExpiresAt}
}

func loginBody(res models.LoginResult) gen.LoginResponse {
	data := gen.LoginResult{User: userToGen(res.User), CsrfToken: res.CSRFToken, ExpiresAt: res.ExpiresAt}
	return gen.LoginResponse{Success: true, Message: ptr(messageLogin), Data: &data}
}

type mfaLoginResponse struct {
	body    gen.LoginResponse
	cookies []string
}

func (r mfaLoginResponse) VisitCompleteMfaLoginResponse(w http.ResponseWriter) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r.body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	for _, c := range r.cookies {
		w.Header().Add("Set-Cookie", c)
	}
	w.WriteHeader(http.StatusOK)
	_, err := buf.WriteTo(w)
	return err
}

func (c *Controller) CompleteMfaLogin(ctx context.Context, req gen.CompleteMfaLoginRequestObject) (gen.CompleteMfaLoginResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	in := models.MfaLoginInput{Method: string(req.Body.Method)}
	if req.Body.Code != nil {
		in.Code = *req.Body.Code
	}
	if req.Body.Credential != nil {
		in.Credential = *req.Body.Credential
	}
	res, err := c.service.CompleteMfa(ctx, in)
	if err != nil {
		return nil, err
	}
	return mfaLoginResponse{body: loginBody(res), cookies: []string{
		auth.SessionCookie(config.SessionCookieName, res.CookieValue, config.SessionAbsoluteTimeout),
		auth.ClearedSessionCookie(config.MfaCookieName),
	}}, nil
}

func (c *Controller) BeginMfaPasskey(ctx context.Context, _ gen.BeginMfaPasskeyRequestObject) (gen.BeginMfaPasskeyResponseObject, error) {
	res, err := c.service.BeginMfaPasskey(ctx)
	if err != nil {
		return nil, err
	}
	data := gen.PasskeyOptions{Options: res.Options, ExpiresAt: res.ExpiresAt}
	return gen.BeginMfaPasskey200JSONResponse{Success: true, Message: ptr(messageMfaKeyOptions), Data: &data}, nil
}

func (c *Controller) BeginPasskeyLogin(ctx context.Context, _ gen.BeginPasskeyLoginRequestObject) (gen.BeginPasskeyLoginResponseObject, error) {
	res, err := c.service.BeginPasskeyLogin(ctx)
	if err != nil {
		return nil, err
	}
	return gen.BeginPasskeyLogin200JSONResponse{Success: true, Message: ptr(messageKeyOptions), Data: challengeToGen(res)}, nil
}

func (c *Controller) PasskeyLogin(ctx context.Context, req gen.PasskeyLoginRequestObject) (gen.PasskeyLoginResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.PasskeyLogin(ctx, req.Body.ChallengeId, req.Body.Credential)
	if err != nil {
		return nil, err
	}
	return gen.PasskeyLogin200JSONResponse{
		Body:    loginBody(res),
		Headers: gen.PasskeyLogin200ResponseHeaders{SetCookie: auth.SessionCookie(config.SessionCookieName, res.CookieValue, config.SessionAbsoluteTimeout)},
	}, nil
}

func (c *Controller) GetMfaStatus(ctx context.Context, _ gen.GetMfaStatusRequestObject) (gen.GetMfaStatusResponseObject, error) {
	st, err := c.service.Status(ctx)
	if err != nil {
		return nil, err
	}
	data := gen.MfaStatus{
		Enabled: st.Enabled, TotpEnabled: st.TotpEnabled, TotpEnabledAt: st.TotpEnabledAt,
		RecoveryCodesRemaining: st.RecoveryCodesRemaining, PasskeyCount: st.PasskeyCount,
	}
	return gen.GetMfaStatus200JSONResponse{Success: true, Message: ptr(messageMfaStatus), Data: &data}, nil
}

func (c *Controller) BeginStepUpPasskey(ctx context.Context, _ gen.BeginStepUpPasskeyRequestObject) (gen.BeginStepUpPasskeyResponseObject, error) {
	res, err := c.service.BeginStepUp(ctx)
	if err != nil {
		return nil, err
	}
	return gen.BeginStepUpPasskey200JSONResponse{Success: true, Message: ptr(messageKeyOptions), Data: challengeToGen(res)}, nil
}

func (c *Controller) StartTotpEnrollment(ctx context.Context, _ gen.StartTotpEnrollmentRequestObject) (gen.StartTotpEnrollmentResponseObject, error) {
	res, err := c.service.StartTotp(ctx)
	if err != nil {
		return nil, err
	}
	data := gen.TotpEnrollment{Secret: res.Secret, Uri: res.URI, ExpiresAt: res.ExpiresAt}
	return gen.StartTotpEnrollment200JSONResponse{Success: true, Message: ptr(messageTotpStarted), Data: &data}, nil
}

func (c *Controller) ConfirmTotp(ctx context.Context, req gen.ConfirmTotpRequestObject) (gen.ConfirmTotpResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.ConfirmTotp(ctx, req.Body.Code, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	return gen.ConfirmTotp200JSONResponse{Success: true, Message: ptr(messageTotpEnabled), Data: changeToGen(res)}, nil
}

func (c *Controller) DisableTotp(ctx context.Context, req gen.DisableTotpRequestObject) (gen.DisableTotpResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.DisableTotp(ctx, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	return gen.DisableTotp200JSONResponse{Success: true, Message: ptr(messageTotpDisabled), Data: changeToGen(res)}, nil
}

func (c *Controller) RegenerateRecoveryCodes(ctx context.Context, req gen.RegenerateRecoveryCodesRequestObject) (gen.RegenerateRecoveryCodesResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.RegenerateRecoveryCodes(ctx, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	return gen.RegenerateRecoveryCodes200JSONResponse{Success: true, Message: ptr(messageRecoveryNew), Data: changeToGen(res)}, nil
}

func (c *Controller) ListPasskeys(ctx context.Context, _ gen.ListPasskeysRequestObject) (gen.ListPasskeysResponseObject, error) {
	list, err := c.service.ListPasskeys(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]gen.Passkey, 0, len(list))
	for _, p := range list {
		items = append(items, passkeyToGen(p))
	}
	return gen.ListPasskeys200JSONResponse{Success: true, Message: ptr(messageKeysListed), Data: &items}, nil
}

func (c *Controller) BeginPasskeyRegistration(ctx context.Context, _ gen.BeginPasskeyRegistrationRequestObject) (gen.BeginPasskeyRegistrationResponseObject, error) {
	res, err := c.service.BeginRegistration(ctx)
	if err != nil {
		return nil, err
	}
	return gen.BeginPasskeyRegistration200JSONResponse{Success: true, Message: ptr(messageKeyOptions), Data: challengeToGen(res)}, nil
}

func (c *Controller) AddPasskey(ctx context.Context, req gen.AddPasskeyRequestObject) (gen.AddPasskeyResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.AddPasskey(ctx, models.AddPasskeyInput{
		ChallengeID: req.Body.ChallengeId, Credential: req.Body.Credential, Name: req.Body.Name,
		Factor: factorOf(req.Body.CurrentFactor),
	})
	if err != nil {
		return nil, err
	}
	change := changeToGen(res.Change)
	data := gen.AddPasskeyResult{Passkey: passkeyToGen(res.Passkey), Revoked: change.Revoked, RecoveryCodes: change.RecoveryCodes}
	return gen.AddPasskey200JSONResponse{Success: true, Message: ptr(messageKeyAdded), Data: &data}, nil
}

func (c *Controller) RenamePasskey(ctx context.Context, req gen.RenamePasskeyRequestObject) (gen.RenamePasskeyResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.RenamePasskey(ctx, req.Id, req.Body.Name)
	if err != nil {
		return nil, err
	}
	data := passkeyToGen(res)
	return gen.RenamePasskey200JSONResponse{Success: true, Message: ptr(messageKeyRenamed), Data: &data}, nil
}

func (c *Controller) RemovePasskey(ctx context.Context, req gen.RemovePasskeyRequestObject) (gen.RemovePasskeyResponseObject, error) {
	if req.Body == nil {
		return nil, apperrors.BadRequest()
	}
	res, err := c.service.RemovePasskey(ctx, req.Id, factorOf(req.Body.CurrentFactor))
	if err != nil {
		return nil, err
	}
	return gen.RemovePasskey200JSONResponse{Success: true, Message: ptr(messageKeyRemoved), Data: changeToGen(res)}, nil
}
