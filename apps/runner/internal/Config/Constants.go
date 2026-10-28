package config

import "time"

const (
	APIPrefix               = "/runner-api"
	ProtocolHeader          = "Dhow-Protocol"
	ProtocolVersion         = "1"
	VersionHeader           = "Dhow-Runner-Version"
	AccessTokenPrefix       = "dhow_rat_"
	RegistrationTokenPrefix = "dhow_rrt_" //nolint:gosec // A token prefix, not a credential.
	MaxRegistrationToken    = 256
)

const (
	DefaultStateDir  = "/var/lib/dhow-runner"
	IdentityFileName = "identity"
	JournalFileName  = "journal"
	AttemptsDirName  = "attempts"
	SpillFileName    = "logs.spill"
	PrivateFileMode  = 0o600
	PrivateDirMode   = 0o700
)

const (
	LeaseWait           = 25 * time.Second
	LeasePollSlack      = 10 * time.Second
	RPCTimeout          = 15 * time.Second
	AppendTimeout       = 30 * time.Second
	HeartbeatInterval   = 10 * time.Second
	SelfFenceMargin     = 10 * time.Second
	MinLeaseTTL         = 2 * SelfFenceMargin
	MaxLeaseTTL         = 5 * time.Minute
	HeartbeatsPerWindow = 3
	BackoffMin          = time.Second
	BackoffMax          = 30 * time.Second
	TokenRenewNumerator = 2
	TokenRenewDivisor   = 3
	CancelKillWait      = 5 * time.Second
	ShutdownGrace       = 15 * time.Second
	CompleteRetryWindow = 10 * time.Minute
	ImagePullTimeout    = 10 * time.Minute
	LogDrainTimeout     = 30 * time.Second
)

const (
	LogChunkBytes       = 64 << 10
	LogFlushInterval    = 250 * time.Millisecond
	LogMemoryQueueBytes = 8 << 20
	LogSpillBytes       = 256 << 20
	LogBatchBytes       = 768 << 10
	LogBatchChunks      = 1024
)

const (
	DriverPodman     = "podman"
	DriverKubernetes = "kubernetes"
	DriverFake       = "fake"

	DiskModeTmpfs     = "tmpfs"
	DiskModeDirectory = "directory"
)

const (
	MinCapacity        = 1
	MaxCapacity        = 16
	DefaultCapacity    = 1
	DefaultMaxTimeout  = 6 * time.Hour
	MinTimeout         = time.Minute
	MaxTimeout         = 24 * time.Hour
	DefaultCancelGrace = 10 * time.Second
	MinCancelGrace     = time.Second
	MaxCancelGrace     = 60 * time.Second
	MillisPerCPU       = 1000
)
