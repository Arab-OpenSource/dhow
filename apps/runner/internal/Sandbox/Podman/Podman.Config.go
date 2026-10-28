package podman

import (
	"fmt"
	"os"
	"path/filepath"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
)

const (
	configDirName   = "containers"
	storageDirName  = "storage"
	runrootDirName  = "run"
	seccompFileName = "seccomp.json"
	traverseDirMode = 0o711
)

const containersConfTemplate = `[containers]
default_capabilities = []
default_sysctls = []
no_hosts = true
init = false
log_driver = "none"
netns = "none"
userns = "auto"
ipcns = "private"
utsns = "private"
cgroupns = "private"
pidns = "private"
seccomp_profile = "%s"

[engine]
cgroup_manager = "cgroupfs"
events_logger = "file"
runtime = "crun"
`

const storageConfTemplate = `[storage]
driver = "overlay"
graphroot = "%s"
runroot = "%s"

[storage.options]
pull_options = {enable_partial_images = "false", use_hard_links = "false"}

[storage.options.overlay]
mountopt = "nodev"
`

const registriesConf = `unqualified-search-registries = []
short-name-mode = "disabled"
`

type paths struct {
	configFile    string
	storageFile   string
	registryFile  string
	seccompFile   string
	storageDir    string
	runrootDir    string
	containersDir string
	home          string
}

func (p paths) env(binary string) runner {
	return runner{binary: binary, env: []string{
		"CONTAINERS_CONF=" + p.configFile,
		"CONTAINERS_STORAGE_CONF=" + p.storageFile,
		"CONTAINERS_REGISTRIES_CONF=" + p.registryFile,
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=" + p.home,
		"XDG_RUNTIME_DIR=" + p.runrootDir,
		"TMPDIR=" + p.runrootDir,
	}}
}

func writeConfig(stateDir string) (paths, error) {
	dir := filepath.Join(stateDir, configDirName)
	p := paths{
		home:          stateDir,
		containersDir: dir,
		configFile:    filepath.Join(dir, "containers.conf"),
		storageFile:   filepath.Join(dir, "storage.conf"),
		registryFile:  filepath.Join(dir, "registries.conf"),
		seccompFile:   filepath.Join(dir, seccompFileName),
		storageDir:    filepath.Join(stateDir, storageDirName),
		runrootDir:    filepath.Join(stateDir, runrootDirName),
	}
	if err := config.EnsurePrivateDir(dir); err != nil {
		return paths{}, err
	}
	for _, d := range []string{p.storageDir, p.runrootDir} {
		if err := config.EnsurePrivateDir(d); err != nil {
			return paths{}, err
		}
	}
	for _, d := range []string{stateDir, p.runrootDir} {
		if err := os.Chmod(d, traverseDirMode); err != nil {
			return paths{}, fmt.Errorf("let the job user namespace traverse %s: %w", d, err)
		}
	}
	files := map[string][]byte{
		p.seccompFile:  seccompProfile,
		p.configFile:   []byte(fmt.Sprintf(containersConfTemplate, p.seccompFile)),
		p.storageFile:  []byte(fmt.Sprintf(storageConfTemplate, p.storageDir, p.runrootDir)),
		p.registryFile: []byte(registriesConf),
	}
	for name, body := range files {
		if err := writeFile(name, body); err != nil {
			return paths{}, err
		}
	}
	return p, nil
}

func writeFile(path string, body []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, config.PrivateFileMode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("install %s: %w", path, err)
	}
	return nil
}
