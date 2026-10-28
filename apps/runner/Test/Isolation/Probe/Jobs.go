//go:build linux

package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func markerDirs() []string { return []string{"/workspace", "/tmp", "/dev/shm"} }

func marker(token string, hold time.Duration) int {
	if token == "" {
		return exitUsage
	}
	for _, dir := range markerDirs() {
		if err := os.WriteFile(filepath.Join(dir, "marker-"+token), []byte(token), 0o644); err != nil { //nolint:gosec // A marker another job must not see.
			return exitUsage
		}
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "unix", "@dhow-marker-"+token)
	if err != nil {
		return exitUsage
	}
	defer func() { _ = ln.Close() }()
	time.Sleep(hold)
	return 0
}

func hunt(token string) []result {
	if token == "" {
		return []result{escaped("the other job", "no token to look for")}
	}
	out := make([]result, 0, len(markerDirs())+2)
	for _, dir := range markerDirs() {
		out = append(out, foundIn("the other job's "+dir, dir, token))
	}
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "unix", "@dhow-marker-"+token)
	if err == nil {
		_ = conn.Close()
		out = append(out, escaped("the other job's abstract socket", "connected to it"))
	} else {
		out = append(out, blocked("the other job's abstract socket"))
	}
	return append(out, processes(token))
}

func foundIn(name, dir, token string) result {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return blocked(name)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), token) || containsToken(filepath.Join(dir, e.Name()), token) {
			return escaped(name, "found %s", e.Name())
		}
	}
	return blocked(name)
}

func processes(token string) result {
	self := "/proc/" + strconv.Itoa(os.Getpid()) + "/cmdline"
	paths, _ := filepath.Glob("/proc/[0-9]*/cmdline")
	for _, p := range paths {
		if p != self && containsToken(p, token) {
			return escaped("the other job's processes", "%s is visible", p)
		}
	}
	return blocked("the other job's processes")
}
