package config

import (
	"errors"
	"fmt"

	"github.com/Khaymah/dhow/internal/jobspec"
)

type JobMemory struct {
	MemoryBytes int64
	DiskBytes   int64
	Tmpfs       bool
}

func DefaultJobMemory() JobMemory {
	return JobMemory{MemoryBytes: jobspec.DefaultMemoryBytes, DiskBytes: jobspec.DefaultDiskBytes, Tmpfs: true}
}

func (j JobMemory) Total() int64 {
	if j.Tmpfs {
		return j.MemoryBytes + j.DiskBytes
	}
	return j.MemoryBytes
}

func ReadJobMemory(getenv func(string) string) (JobMemory, error) {
	mode, _ := oneOf(getenv(EnvDiskMode), DiskModeTmpfs, ErrUnknownDisk, DiskModeTmpfs, DiskModeDirectory)
	j, errs := parseJobMemory(getenv, mode)
	return j, errors.Join(errs...)
}

func parseJobMemory(getenv func(string) string, diskMode string) (JobMemory, []error) {
	var errs []error
	j := JobMemory{Tmpfs: diskMode == DiskModeTmpfs}
	var err error
	j.MemoryBytes, err = parseBytesRange(getenv(EnvMaxMemory), jobspec.DefaultMemoryBytes, jobspec.MinMemoryBytes, jobspec.MaxMemoryBytes)
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvMaxMemory, err))
	}
	j.DiskBytes, err = parseBytesRange(getenv(EnvMaxDisk), jobspec.DefaultDiskBytes, jobspec.MinDiskBytes, jobspec.MaxDiskBytes)
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvMaxDisk, err))
	}
	return j, errs
}

func CheckJobMemory(j JobMemory, machineBytes int64) error {
	if machineBytes <= 0 || j.Total() <= machineBytes {
		return nil
	}
	return fmt.Errorf("%s: %d bytes of memory per job (with the tmpfs disk) %w (%d); lower %s or %s",
		EnvMaxMemory, j.Total(), ErrAboveMachine, machineBytes, EnvMaxMemory, EnvMaxDisk)
}
