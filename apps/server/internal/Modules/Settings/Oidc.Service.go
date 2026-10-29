package settings

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	oidcauth "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc"
	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
	instancesettings "github.com/Khaymah/dhow/apps/server/internal/Core/Settings"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	"github.com/Khaymah/dhow/internal/logging"
)

const (
	limitOidcTest   = "oidc-test"
	targetOidcGroup = instancesettings.GroupOidc + ":"
)

type OidcView struct {
	Snapshot      *instancesettings.Snapshot
	CallbackBase  string
	Linked        map[string]int64
	OnlySignIn    map[string]int64
	IgnoredStored int64
	IgnoredLinked int64
}

func (s *Service) oidcView(ctx context.Context, snap *instancesettings.Snapshot) (OidcView, error) {
	q := sqlc.New(s.db)
	counts, err := q.CountIdentitiesByProvider(ctx)
	if err != nil {
		return OidcView{}, err
	}
	view := OidcView{Snapshot: snap, CallbackBase: s.callbackBase, Linked: map[string]int64{}, OnlySignIn: map[string]int64{}}
	for _, c := range counts {
		view.Linked[c.Provider] = c.Identities
	}
	if snap.Oidc.Locked() {
		return s.lockedView(ctx, q, view)
	}
	enabled := []string{}
	for _, p := range snap.Oidc.Providers {
		if p.Enabled() {
			enabled = append(enabled, p.Config.ID)
		}
	}
	for _, p := range snap.Oidc.Providers {
		if !p.Enabled() || view.Linked[p.Config.ID] == 0 {
			continue
		}
		others := make([]string, 0, len(enabled))
		for _, id := range enabled {
			if id != p.Config.ID {
				others = append(others, id)
			}
		}
		n, err := q.CountUsersOnlyOnProvider(ctx, sqlc.CountUsersOnlyOnProviderParams{Provider: p.Config.ID, Others: others})
		if err != nil {
			return OidcView{}, err
		}
		view.OnlySignIn[p.Config.ID] = n
	}
	return view, nil
}

func (s *Service) lockedView(ctx context.Context, q *sqlc.Queries, view OidcView) (OidcView, error) {
	stored, err := q.CountOidcProviders(ctx)
	if err != nil {
		return OidcView{}, err
	}
	active := make([]string, 0, len(view.Snapshot.Oidc.Providers))
	for _, p := range view.Snapshot.Oidc.Providers {
		active = append(active, p.Config.ID)
	}
	stranded, err := q.CountStrandedIdentities(ctx, active)
	if err != nil {
		return OidcView{}, err
	}
	view.IgnoredStored, view.IgnoredLinked = stored, stranded.Identities
	return view, nil
}

func (s *Service) GetOidc(ctx context.Context) (OidcView, error) {
	if _, err := owner(ctx); err != nil {
		return OidcView{}, err
	}
	return s.oidcView(ctx, s.store.Current())
}

func (s *Service) ChangeOidc(ctx context.Context, c instancesettings.OidcChange, factor auth.Factor) (OidcView, error) {
	p, err := owner(ctx)
	if err != nil {
		return OidcView{}, err
	}
	if err := s.store.CheckOidc(ctx, c); err != nil {
		return OidcView{}, err
	}
	if err := s.confirm(ctx, p, factor); err != nil {
		return OidcView{}, err
	}
	res, err := s.store.ApplyOidc(ctx, c, p.UserID)
	if err != nil {
		return OidcView{}, err
	}
	if len(res.Changed) > 0 {
		if err := s.auditOidc(ctx, p.UserID, c.Operation, res); err != nil {
			return OidcView{}, err
		}
	}
	return s.oidcView(ctx, res.Snapshot)
}

func (s *Service) auditOidc(ctx context.Context, actor uuid.UUID, op instancesettings.OidcOperation, res instancesettings.OidcResult) error {
	fields := strings.Join(res.Changed, ",")
	if len(fields) > fieldListMaxLen {
		fields = fields[:fieldListMaxLen]
	}
	return s.bus.Publish(ctx, eventbus.Event{Name: eventbus.SystemSettingsChanged, Payload: eventbus.Audited{
		ActorUserID: &actor, TargetType: targetSettings, TargetID: targetOidcGroup + res.ID, IP: auth.RequestMetaFrom(ctx).IPString(),
		Data: map[string]string{
			"Group": instancesettings.GroupOidc, "Provider": res.ID, "Operation": string(op), "Fields": fields,
			"Version": strconv.FormatInt(res.Version, 10),
		},
	}})
}

func (s *Service) TestOidc(ctx context.Context, kind, issuer string) (oidcauth.ProbeResult, error) {
	p, err := owner(ctx)
	if err != nil {
		return oidcauth.ProbeResult{}, err
	}
	d, err := s.limiter.Take(ctx, ratelimit.Limit{Name: limitOidcTest, Max: config.OidcTestPerWindow, Window: config.OidcTestWindow}, p.UserID.String())
	if err != nil {
		return oidcauth.ProbeResult{}, err
	}
	if err := d.Err(); err != nil {
		return oidcauth.ProbeResult{}, err
	}
	k, e := settingspec.ParseOIDCKind(kind)
	if e == nil {
		issuer, e = settingspec.ParseOIDCIssuer(issuer, k, s.dev)
	}
	if e != nil {
		return oidcauth.ProbeResult{}, apperrors.SettingInvalid(apperrors.IErrorDetail{Field: e.Field, Message: "settings.invalid." + e.Rule})
	}
	res := s.providers.Probe(ctx, k, issuer)
	if res.Err != nil {
		s.log.WarnContext(ctx, "the sign-in provider test failed", "issuer", logging.UserValue(issuer), "reason", res.Reason())
	}
	return res, nil
}
