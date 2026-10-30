package config

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

const EnvRunnerMaxPolls = "DHOW_RUNNER_MAX_POLLS"

const (
	JobLeaseTTL              = 45 * time.Second
	JobLeaseGrace            = 5 * time.Second
	JobDeadlineGrace         = 30 * time.Second
	JobQueueTimeout          = 24 * time.Hour
	JobReconcileInterval     = 5 * time.Second
	JobReconcileBatch        = 100
	JobMaxPollWait           = 25 * time.Second
	JobPollFallback          = 5 * time.Second
	JobPollRetryAfter        = 5 * time.Second
	JobQueuedChannel         = "dhow_jobs_queued"
	JobListenRetryMin        = time.Second
	JobListenRetryMax        = 30 * time.Second
	JobBodyMaxBytes          = 512 << 10
	JobNameMaxLength         = 100
	JobPriorityMax           = 100
	JobDefaultInfraRetries   = 2
	JobMaxInfraRetries       = 5
	JobStartDeadline         = 10 * time.Minute
	JobLeasedCancelGrace     = time.Minute
	JobClaimBurst            = 5
	JobClaimRefill           = time.Second
	JobClaimTrackedRunners   = 10_000
	JobMaxQueuedPerOrg       = 1000
	JobMaxQueuedWithFeatures = 100
	JobRecentFailureWindow   = 24 * time.Hour
	RunnerDefaultMaxPolls    = DatabaseMaxConns * 25
	RunnerMaxPollsUpperBound = 10_000
)

func parseMaxPolls(v string) (int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return RunnerDefaultMaxPolls, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > RunnerMaxPollsUpperBound {
		return 0, errors.New("must be a whole number from 1 to " + strconv.Itoa(RunnerMaxPollsUpperBound))
	}
	return n, nil
}
