//go:build unix

package safefile

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadRegularNeverBlocksOnAFifo(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "fifo")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	start := time.Now()
	_, err := ReadRegular(fifo, 10)
	assert.ErrorIs(t, err, ErrNotRegular)
	assert.Less(t, time.Since(start), time.Second)
	_, err = ReadRegular(os.DevNull, 10)
	assert.ErrorIs(t, err, ErrNotRegular)
}
