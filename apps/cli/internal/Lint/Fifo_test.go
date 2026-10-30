//go:build unix

package lint

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFifoIsRefusedWithoutBlocking(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), ".dhow.yml")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	start := time.Now()
	_, err := File(fifo)
	assert.ErrorIs(t, err, ErrNotRegular)
	assert.Less(t, time.Since(start), time.Second)
}
