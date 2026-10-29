package config

import (
	"time"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

const (
	EnvRateLimitPublic      = settingspec.EnvRateLimitPublic
	EnvRateLimitInvitesUser = settingspec.EnvRateLimitInvitesUser
	EnvRateLimitInvitesOrg  = settingspec.EnvRateLimitInvitesOrg
)

const (
	RateLimitPublicWindow       = settingspec.RateLimitPublicWindow
	RateLimitPublicDefault      = settingspec.RateLimitPublicDefault
	RateLimitInviteWindow       = settingspec.RateLimitInviteWindow
	RateLimitInvitesUserDefault = settingspec.RateLimitInvitesUserDefault
	RateLimitInvitesOrgDefault  = settingspec.RateLimitInvitesOrgDefault
	RateLimitUpperBound         = settingspec.RateLimitUpperBound
	RateLimitTrackedKeys        = 10_000
	InvitationMaxSends          = 5
	MailTestPerWindow           = 5
	MailTestWindow              = 10 * time.Minute
	OidcTestPerWindow           = 5
	OidcTestWindow              = 10 * time.Minute
)

type RateLimits = settingspec.RateLimits
