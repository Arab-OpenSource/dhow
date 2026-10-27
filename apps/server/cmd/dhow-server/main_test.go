package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunRejectsUnknownCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown", []string{"serve"}},
		{"healthcheck with extra arguments", []string{"healthcheck", "--now"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(context.Background(), tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unknown command")
		})
	}
}

func TestRunHealthcheckFailsWhenNothingListens(t *testing.T) {
	t.Setenv("DHOW_LISTEN_ADDR", "127.0.0.1:1")
	require.Error(t, run(context.Background(), []string{"healthcheck"}))
}
