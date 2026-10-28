package config

import "time"

const (
	RetentionEvery          = 10 * time.Minute
	RetentionBatch          = 1000
	RetentionMaxBatches     = 50
	RetentionLockKey        = int64(0x64686f772d726574)
	RetentionThrottleAfter  = 24 * time.Hour
	RetentionChallengeAfter = 24 * time.Hour
	RetentionOidcFlowAfter  = 24 * time.Hour
	RetentionSetupAfter     = 24 * time.Hour
	RetentionInviteAfter    = 30 * 24 * time.Hour
	RetentionRateLimitAfter = time.Hour
)
