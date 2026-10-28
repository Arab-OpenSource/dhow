package podman

import (
	"errors"
	"testing"
)

func TestTheUserNamespaceMapIsChecked(t *testing.T) {
	const runner = 1001
	other := [][]idRange{{{inside: 0, outside: 200000, count: 65536}}}
	tests := []struct {
		name string
		raw  string
		ok   bool
	}{
		{"one private range", "0 100000 65536\n", true},
		{"the runner's own uid", "0 1001 1\n1 100000 65535\n", false},
		{"host root", "0 0 65536\n", false},
		{"too many ids", "0 100000 131072\n", false},
		{"overlaps another sandbox", "0 230000 65536\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges, err := parseIDMap(tt.raw)
			if err == nil {
				err = verifyIDMap(ranges, runner, other)
			}
			if tt.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tt.ok && !errors.Is(err, ErrPolicyMismatch) {
				t.Fatalf("want a policy mismatch, got %v", err)
			}
		})
	}
}
