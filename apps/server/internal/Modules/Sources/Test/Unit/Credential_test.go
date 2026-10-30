package unit_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sources "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources"
)

func TestACredentialNeverPrintsItsSecret(t *testing.T) {
	c := sources.Credential{Kind: "ssh_key", Secret: []byte("very-private-key-material"), KnownHost: "git.example.com ssh-ed25519 AAAA"}
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("minted", "credential", c, "pointer", &c)
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	outputs := []string{
		buf.String(), string(raw), fmt.Sprint(c), fmt.Sprintf("%v %+v %#v %s %q %x", c, c, c, c, c, c), fmt.Sprintf("%v", &c), c.String(),
	}
	for _, out := range outputs {
		assert.NotContains(t, out, "very-private-key-material")
		assert.NotContains(t, out, "7665727")
	}
	assert.Contains(t, buf.String(), `"Secret":"[redacted]"`)
	assert.Contains(t, buf.String(), `"Kind":"ssh_key"`)
	assert.Equal(t, []byte("very-private-key-material"), c.Secret)
}
