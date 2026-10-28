package unit_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	preferences "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Preferences"
)

const themesFile = "../../../../../../web/Src/Styles/Themes.json"

func TestTheServerKnowsTheSameThemesAsTheWeb(t *testing.T) {
	raw, err := os.ReadFile(themesFile)
	require.NoError(t, err)
	var themes struct {
		Default string
		Tokens  []struct{ Key string }
		Themes  []struct{ Id string }
	}
	require.NoError(t, json.Unmarshal(raw, &themes))
	keys := make([]string, 0, len(themes.Tokens))
	for _, tok := range themes.Tokens {
		keys = append(keys, tok.Key)
	}
	ids := make([]string, 0, len(themes.Themes))
	for _, th := range themes.Themes {
		ids = append(ids, th.Id)
	}
	assert.Equal(t, keys, preferences.ThemeTokens())
	assert.Equal(t, ids, preferences.SystemThemes())
	assert.Equal(t, themes.Default, config.DefaultThemeID)
}
