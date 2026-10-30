package config

import (
	"errors"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/Khaymah/dhow/internal/netguard"
)

const (
	EnvWebhookMaxBytes       = "DHOW_WEBHOOK_MAX_BYTES"
	EnvGitAllowPrivateCIDRs  = "DHOW_GIT_ALLOW_PRIVATE_CIDRS"
	WebhookPrefix            = "/webhooks"
	WebhookMaxBytesDefault   = 5 << 20
	WebhookMaxBytesMin       = 64 << 10
	WebhookMaxBytesMax       = 25 << 20
	WebhookReplayWindow      = 5 * time.Minute
	WebhookDedupeWindow      = 72 * time.Hour
	WebhookRetentionDays     = 30
	WebhookDigestWindow      = (WebhookRetentionDays + 1) * 24 * time.Hour
	WebhookPartitionsAhead   = 3
	WebhookMaintainInterval  = time.Hour
	WebhookMaintainLockWait  = 2 * time.Second
	WebhookTimeout           = 15 * time.Second
	WebhookPerIPMax          = 1200
	WebhookPerHookMax        = 300
	WebhookFailuresPerHook   = 100
	WebhookInflightDefault   = 64 << 20
	WebhookDrainBytes        = 16 << 10
	WebhookBusyRetry         = 2 * time.Second
	WebhookRateWindow        = time.Minute
	WebhookSecretBytes       = 32
	WebhookHookIDBytes       = 16
	WebhookDeliveryKeyLabel  = "dhow/git-webhook-secret/v1"
	DeployKeySealLabel       = "dhow/git-deploy-key/v1"
	DeployKeyComment         = "dhow-deploy-key"
	DeployKeyRotationGrace   = 24 * time.Hour
	HostKeyFetchTimeout      = 10 * time.Second
	GitDialPerUser           = 30
	GitDialPerOrg            = 120
	GitDialWindow            = time.Hour
	CommitStatusInterval     = 5 * time.Second
	CommitStatusBatch        = 20
	CommitStatusClaim        = 2 * time.Minute
	CommitStatusBaseBackoff  = 10 * time.Second
	CommitStatusMaxBackoff   = time.Hour
	CommitStatusGiveUpAfter  = 24 * time.Hour
	SourcesMaintainBatchSize = 500
)

type Sources struct {
	WebhookMaxBytes      int64
	WebhookInflightBytes int64
	AllowPrivate         []netip.Prefix
}

func DefaultSources() Sources {
	return Sources{WebhookMaxBytes: WebhookMaxBytesDefault, WebhookInflightBytes: WebhookInflightDefault}
}

func loadSources(getenv func(string) string) (Sources, []error) {
	out := DefaultSources()
	var errs []error
	if v := strings.TrimSpace(getenv(EnvWebhookMaxBytes)); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < WebhookMaxBytesMin || n > WebhookMaxBytesMax {
			errs = append(errs, errors.New(EnvWebhookMaxBytes+": must be a whole number of bytes from "+
				strconv.Itoa(WebhookMaxBytesMin)+" to "+strconv.Itoa(WebhookMaxBytesMax)))
		} else {
			out.WebhookMaxBytes = n
		}
	}
	allowed, err := netguard.ParseCIDRs(getenv(EnvGitAllowPrivateCIDRs))
	if err != nil {
		errs = append(errs, errors.New(EnvGitAllowPrivateCIDRs+": "+err.Error()))
	} else {
		out.AllowPrivate = allowed
	}
	return out, errs
}
