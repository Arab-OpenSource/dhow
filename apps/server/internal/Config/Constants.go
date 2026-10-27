package config

import "time"

const (
	DefaultListenAddr = "127.0.0.1:8080"
	MinSecretKeyBytes = 32
)

const (
	MaxBodyBytes = 1 << 20
	APITimeout   = 15 * time.Second
)

const (
	ReadHeaderTimeout = 5 * time.Second
	ReadTimeout       = 30 * time.Second
	WriteTimeout      = 60 * time.Second
	IdleTimeout       = 120 * time.Second
	MaxHeaderBytes    = 64 << 10
	ShutdownTimeout   = 20 * time.Second
)

const HealthcheckTimeout = 3 * time.Second

const (
	DatabaseMaxConns    = 20
	DatabaseMaxIdleTime = 5 * time.Minute
	DatabaseConnTimeout = 10 * time.Second
	DatabaseAppName     = "dhow-server"
)

const (
	APIPrefix = "/api/v1"

	SessionCookieName      = "__Host-dhow_session"
	CSRFHeaderName         = "X-CSRF-Token"
	SessionIdleTimeout     = 12 * time.Hour
	SessionAbsoluteTimeout = 14 * 24 * time.Hour
	SessionTouchInterval   = time.Minute
	SessionTokenBytes      = 32
)

const (
	MfaCookieName           = "__Host-dhow_mfa"
	MfaPendingTTL           = 5 * time.Minute
	MfaMaxChallengeAttempts = 5
	MfaFreeAttempts         = 5
	TotpEnrollmentTTL       = 10 * time.Minute
	TotpIssuer              = "Dhow"
	TotpSealLabel           = "dhow/totp-secret/v1"
	WebAuthnChallengeTTL    = 5 * time.Minute
	WebAuthnDisplayName     = "Dhow"
	WebAuthnHandleBytes     = 32
	PasskeyNameMaxLength    = 64
	MfaCodeMaxBytes         = 64
)

const (
	SetupTokenTTL   = time.Hour
	SetupTokenBytes = 32
)

const (
	PasswordMinLength = 12
	PasswordMaxLength = 128
	AuthBodyMaxBytes  = 16 << 10
)

const (
	AccountFreeAttempts = 5
	IPFreeAttempts      = 20
	ThrottleBaseLock    = 30 * time.Second
	ThrottleMaxLock     = 15 * time.Minute
	ThrottleResetAfter  = time.Hour
)

const (
	BreachedCheckTimeout = 3 * time.Second
	BreachedRangeURL     = "https://api.pwnedpasswords.com/range/"
)

const (
	DefaultPageLimit = 25
	MaxPageLimit     = 100
)
