package config

import "os"

const (
	EnvTestDatabaseURL = "DHOW_TEST_DATABASE_URL"
	envCI              = "CI"
)

func TestDatabaseURL() string { return os.Getenv(EnvTestDatabaseURL) }

func RunningInCI() bool { return os.Getenv(envCI) != "" }
