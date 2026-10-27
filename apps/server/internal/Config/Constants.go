package config

import "time"

const (
	DefaultListenAddr = "127.0.0.1:8080"
	MinSecretKeyBytes = 32
)

const (
	MaxBodyBytes = 1 << 20
	APITimeout   = 15 * time.Second
)

const (
	ReadHeaderTimeout = 5 * time.Second
	ReadTimeout       = 30 * time.Second
	WriteTimeout      = 60 * time.Second
	IdleTimeout       = 120 * time.Second
	MaxHeaderBytes    = 64 << 10
	ShutdownTimeout   = 20 * time.Second
)

const HealthcheckTimeout = 3 * time.Second

const (
	DatabaseMaxConns    = 20
	DatabaseMaxIdleTime = 5 * time.Minute
	DatabaseConnTimeout = 10 * time.Second
	DatabaseAppName     = "dhow-server"
)
