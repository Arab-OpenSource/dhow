package buildinfo

import "testing"

func TestGetDefaults(t *testing.T) {
	info := Get()
	if info.Version != "dev" {
		t.Fatalf("version = %q, want %q", info.Version, "dev")
	}
	if info.GoVersion == "" {
		t.Fatal("GoVersion is empty")
	}
}
