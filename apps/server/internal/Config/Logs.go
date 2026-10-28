package config

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	EnvJobLogMaxBytes    = "DHOW_JOB_LOG_MAX_BYTES"
	EnvLogRetentionDays  = "DHOW_LOG_RETENTION_DAYS"
	EnvSSEMaxStreams     = "DHOW_SSE_MAX_STREAMS"
	EnvLogReadConns      = "DHOW_LOG_READ_CONNS"
	LogAppendedChannel   = "dhow_log_appended"
	LogChunkMaxBytes     = 64 << 10
	LogAppendMaxBytes    = RunnerBodyMaxBytes
	LogAppendMaxChunks   = 1024
	LogMaxStepIndex      = 99
	LogPageMaxBytes      = 1 << 20
	LogPageDefaultLimit  = 100
	LogPageMaxLimit      = 500
	LogStreamBatchRows   = 64
	LogStreamBatchBytes  = 256 << 10
	LogStreamTick        = 15 * time.Second
	LogStreamWriteWait   = 10 * time.Second
	LogStreamRetry       = 3 * time.Second
	LogStreamsPerUser    = 10
	LogStreamRetryAfter  = 5 * time.Second
	LogPartitionsAhead   = 3
	LogMaintainInterval  = time.Hour
	LogMaintainLockWait  = 2 * time.Second
	LogMaxBytesDefault   = 64 << 20
	LogMaxBytesMin       = 1 << 20
	LogMaxBytesMax       = 1 << 30
	LogRetentionDefault  = 30
	LogRetentionMax      = 3650
	SSEMaxStreamsDefault = 2000
	SSEMaxStreamsMax     = 100_000
	LogReadConnsDefault  = 4
	LogReadStatementWait = 5 * time.Second
	LogReadWait          = 10 * time.Second
	LogOrgStreamsShare   = 10
	LogReadBurst         = 30
	LogReadRefill        = 200 * time.Millisecond
	LogStreamOpenBurst   = 20
	LogStreamOpenRefill  = time.Second
	LogTrackedUsers      = 10_000
)

type Logs struct {
	MaxBytes      int64
	RetentionDays int
	MaxStreams    int
	ReadConns     int
}

func DefaultLogs() Logs {
	return Logs{
		MaxBytes: LogMaxBytesDefault, RetentionDays: LogRetentionDefault, MaxStreams: SSEMaxStreamsDefault, ReadConns: LogReadConnsDefault,
	}
}

func (l Logs) OrgStreams() int { return max(2*LogStreamsPerUser, l.MaxStreams/LogOrgStreamsShare) }

func boundedInt(v string, def, lowest, highest int64) (int64, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < lowest || n > highest {
		return 0, errors.New("must be a whole number from " + strconv.FormatInt(lowest, 10) + " to " + strconv.FormatInt(highest, 10))
	}
	return n, nil
}

func loadLogs(getenv func(string) string) (Logs, []error) {
	out := DefaultLogs()
	var errs []error
	maxBytes, err := boundedInt(getenv(EnvJobLogMaxBytes), LogMaxBytesDefault, LogMaxBytesMin, LogMaxBytesMax)
	if err != nil {
		errs = append(errs, errors.New(EnvJobLogMaxBytes+": "+err.Error()))
	}
	retention, err := boundedInt(getenv(EnvLogRetentionDays), LogRetentionDefault, 1, LogRetentionMax)
	if err != nil {
		errs = append(errs, errors.New(EnvLogRetentionDays+": "+err.Error()))
	}
	streams, err := boundedInt(getenv(EnvSSEMaxStreams), SSEMaxStreamsDefault, 1, SSEMaxStreamsMax)
	if err != nil {
		errs = append(errs, errors.New(EnvSSEMaxStreams+": "+err.Error()))
	}
	conns, err := boundedInt(getenv(EnvLogReadConns), LogReadConnsDefault, 1, DatabaseMaxConns)
	if err != nil {
		errs = append(errs, errors.New(EnvLogReadConns+": "+err.Error()))
	}
	if len(errs) > 0 {
		return out, errs
	}
	return Logs{MaxBytes: maxBytes, RetentionDays: int(retention), MaxStreams: int(streams), ReadConns: int(conns)}, nil
}
