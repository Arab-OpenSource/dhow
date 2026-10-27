package buildinfo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaults(t *testing.T) {
	info := Get()
	assert.Equal(t, "dev", info.Version)
	assert.NotEmpty(t, info.GoVersion)
}
