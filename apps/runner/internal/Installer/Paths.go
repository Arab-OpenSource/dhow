package installer

import (
	_ "embed"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	engine "github.com/Khaymah/dhow/internal/installer"
)

const (
	ServiceName       = "dhow-runner"
	ServiceUnit       = ServiceName + ".service"
	ServiceUser       = "dhow-runner"
	BinaryPath        = "/usr/local/bin/dhow-runner"
	UnitPath          = "/etc/systemd/system/" + ServiceUnit
	DropInDir         = "/etc/systemd/system/" + ServiceUnit + ".d"
	CredDropInPath    = DropInDir + "/10-dhow-identity.conf"
	ConfigDir         = "/etc/dhow-runner"
	EnvFilePath       = config.EnvFilePath
	AnswersPath       = ConfigDir + "/answers.yaml"
	StateDir          = config.DefaultStateDir
	IdentityPath      = StateDir + "/" + config.IdentityFileName
	PendingPath       = StateDir + "/" + config.PendingFileName
	SealedPath        = StateDir + "/" + config.SealedIdentityFileName
	SealedPendingPath = StateDir + "/" + config.SealedPendingFileName
	SubUIDPath        = "/etc/subuid"
	SubGIDPath        = "/etc/subgid"
	CgroupControllers = "/sys/fs/cgroup/cgroup.controllers"
	MaxUserNamespaces = "/proc/sys/user/max_user_namespaces"
	AppArmorUserns    = "/proc/sys/kernel/apparmor_restrict_unprivileged_userns"
	SystemdRunDir     = "/run/systemd/system"
)

const (
	SubIDsPerJob = 65536
	SubIDRange   = config.MaxCapacity * SubIDsPerJob
	SubIDFloor   = 1 << 20
	MinPodman    = "5.4.0"
	MaxExposure  = 6.9
	ReadyMessage = "the runner is ready"
)

var (
	//go:embed Assets/dhow-runner.service
	UnitFile []byte

	modeRootPublic  = engine.FileMode{Perm: 0o644}
	modeRootExec    = engine.FileMode{Perm: 0o755}
	modeRootPrivate = engine.FileMode{Perm: 0o600}
	modePublicDir   = engine.FileMode{Perm: 0o755}
	modeConfigDir   = engine.FileMode{Perm: 0o750, Owner: "root", Group: ServiceUser}
	modeEnvFile     = engine.FileMode{Perm: 0o640, Owner: "root", Group: ServiceUser}
	modeStateDir    = engine.FileMode{Perm: 0o700, Owner: ServiceUser, Group: ServiceUser}
)

func CredDropIn() []byte {
	return []byte("[Service]\nLoadCredentialEncrypted=" + config.IdentityCredential + ":" + SealedPath + "\n")
}
