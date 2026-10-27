package config

import (
	"fmt"
	"os"
)

func LoadListenAddrFromEnv() (string, error) { return LoadListenAddr(os.Getenv) }

func LoadListenAddr(getenv func(string) string) (string, error) {
	addr, err := parseListenAddr(getenv(EnvListenAddr))
	if err != nil {
		return "", fmt.Errorf("%s: %w", EnvListenAddr, err)
	}
	return addr, nil
}
