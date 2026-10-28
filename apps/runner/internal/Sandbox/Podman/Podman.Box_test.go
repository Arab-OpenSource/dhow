package podman

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKillingAStoppedOrRemovedContainerIsNotAnError(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		gone    bool
		stopped bool
	}{
		{"removed", errors.New("Error: no such container dhow-1"), true, false},
		{"stopped", errors.New("podman kill: exit status 125: Error: can only kill running containers. abc is in state exited: container state improper"), false, true},
		{"other", errors.New("podman kill: exit status 125: Error: permission denied"), false, false},
		{"none", nil, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.gone, isGone(c.err))
			assert.Equal(t, c.stopped, notRunning(c.err))
		})
	}
}
