package app_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	app "github.com/Arab-OpenSource/dhow/apps/server/internal/App"
)

const openapiFile = "../../../../packages/openapi/openapi.yaml"

type openapiOperation struct {
	Security []map[string][]string `yaml:"security"`
}

func TestOpenAPIListsTheTokenSchemeExactlyOnTokenRoutes(t *testing.T) {
	raw, err := os.ReadFile(openapiFile)
	require.NoError(t, err)
	var spec struct {
		Paths map[string]map[string]openapiOperation `yaml:"paths"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	for _, rule := range app.Policies() {
		op, ok := spec.Paths[rule.Path][strings.ToLower(rule.Method)]
		require.True(t, ok, "%s is not in openapi.yaml", rule.Key())
		listed := false
		for _, req := range op.Security {
			if _, ok := req["ApiToken"]; ok {
				listed = true
			}
		}
		assert.Equal(t, rule.Tokens, listed, "%s: the ApiToken security scheme must be listed exactly when the route accepts tokens", rule.Key())
	}
}
