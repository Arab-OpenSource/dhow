package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	EnvRateLimitPublic      = "DHOW_RATE_LIMIT_PUBLIC"
	EnvRateLimitInvitesUser = "DHOW_RATE_LIMIT_INVITES_USER"
	EnvRateLimitInvitesOrg  = "DHOW_RATE_LIMIT_INVITES_ORG"
)

const (
	RateLimitPublicWindow       = 10 * time.Minute
	RateLimitPublicDefault      = 300
	RateLimitInviteWindow       = time.Hour
	RateLimitInvitesUserDefault = 50
	RateLimitInvitesOrgDefault  = 200
	RateLimitUpperBound         = 100_000
	RateLimitTrackedKeys        = 10_000
	InvitationMaxSends          = 5
)

type RateLimits struct {
	Public      int
	InvitesUser int
	InvitesOrg  int
}

func (r RateLimits) WithDefaults() RateLimits {
	if r.Public <= 0 {
		r.Public = RateLimitPublicDefault
	}
	if r.InvitesUser <= 0 {
		r.InvitesUser = RateLimitInvitesUserDefault
	}
	if r.InvitesOrg <= 0 {
		r.InvitesOrg = RateLimitInvitesOrgDefault
	}
	return r
}

func loadRateLimits(getenv func(string) string) (RateLimits, error) {
	var out RateLimits
	var errs []error
	for _, f := range []struct {
		env  string
		def  int
		into *int
	}{
		{EnvRateLimitPublic, RateLimitPublicDefault, &out.Public},
		{EnvRateLimitInvitesUser, RateLimitInvitesUserDefault, &out.InvitesUser},
		{EnvRateLimitInvitesOrg, RateLimitInvitesOrgDefault, &out.InvitesOrg},
	} {
		n, err := parseRateLimit(getenv(f.env), f.def)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.env, err))
		}
		*f.into = n
	}
	return out, errors.Join(errs...)
}

func parseRateLimit(v string, def int) (int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > RateLimitUpperBound {
		return 0, errors.New("must be a whole number from 1 to " + strconv.Itoa(RateLimitUpperBound))
	}
	return n, nil
}
