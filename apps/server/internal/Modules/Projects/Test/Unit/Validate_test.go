package unit_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	projects "github.com/Khaymah/dhow/apps/server/internal/Modules/Projects"
)

func TestPipelinePaths(t *testing.T) {
	tests := []struct {
		path string
		ok   bool
	}{
		{".dhow.yml", true},
		{"ci/dhow.yaml", true},
		{".github/dhow/build.yml", true},
		{"", false},
		{"/.dhow.yml", false},
		{"../.dhow.yml", false},
		{"ci/../../.dhow.yml", false},
		{"ci/./dhow.yml", false},
		{"ci//dhow.yml", false},
		{"ci\\dhow.yml", false},
		{"ci/dhow.yml\x00", false},
		{"ci/dhow .yml", false},
		{"ci/dhow.json", false},
		{"ci/", false},
		{strings.Repeat("a", 252) + ".yml", false},
		{strings.Repeat("a", 251) + ".yml", true},
		{"\xff.yml", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.ok, projects.ValidPipelinePath(tt.path))
		})
	}
}

func TestSlugsAndForkPolicies(t *testing.T) {
	for _, ok := range []string{"a", "web-shop", "a1-b2", strings.Repeat("a", 63)} {
		assert.True(t, projects.ValidSlug(ok), ok)
	}
	for _, bad := range []string{"", "-a", "a-", "a--b", "A", "a_b", "a.b", strings.Repeat("a", 64), "مشروع"} {
		assert.False(t, projects.ValidSlug(bad), bad)
	}
	assert.True(t, projects.ValidForkPolicy("approval"))
	assert.True(t, projects.ValidForkPolicy("automatic"))
	assert.False(t, projects.ValidForkPolicy("Automatic"))
	assert.False(t, projects.ValidForkPolicy(""))
}
