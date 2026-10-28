package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	LogStdout = "stdout"
	LogStderr = "stderr"
	LogSystem = "system"
)

type LogChunk struct {
	Seq       int64
	Stream    string
	StepIndex *int
	EmittedAt *time.Time
	CreatedAt time.Time
	Text      string
}

type LogPage struct {
	Attempt   int
	AttemptID uuid.UUID
	State     string
	LastSeq   int64
	Truncated bool
	NextSeq   *int64
	Chunks    []LogChunk
}

type LogStatus struct {
	Attempt   int
	State     string
	Truncated bool
}

type LogQuery struct {
	Attempt *int
	FromSeq int64
	Limit   int
}
