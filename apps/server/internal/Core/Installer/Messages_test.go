package installer

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var verbRe = regexp.MustCompile(`%[sdqvt]`)

func TestCatalogsHaveTheSameKeysAndVerbs(t *testing.T) {
	assert.ElementsMatch(t, MessageKeys(LangEn), MessageKeys(LangAr))
	for _, k := range MessageKeys(LangEn) {
		assert.Equal(t, verbRe.FindAllString(enMessages[k], -1), verbRe.FindAllString(arMessages[k], -1), string(k))
		assert.NotEmpty(t, arMessages[k], string(k))
	}
}

func TestTFallsBack(t *testing.T) {
	assert.Equal(t, "nope.key", NewMessages(LangAr).T(Key("nope.key")))
	assert.Equal(t, "Port 80 is already used by another program, so the proxy cannot start. Free it or choose an existing proxy.", NewMessages(LangEn).T(KeyBlockProxyPort, 80))
}

func TestChooseLang(t *testing.T) {
	tests := []struct {
		flag, answers, locale, want string
	}{
		{"", "", "ar_EG.UTF-8", LangAr},
		{"", "", "ar_EG.utf8", LangAr},
		{"", "", "ar_EG.ISO-8859-6", LangEn},
		{"", "", "ar_EG", LangEn},
		{"", "", "en_US.UTF-8", LangEn},
		{"", "", "", LangEn},
		{"", "", "C", LangEn},
		{"ar", "", "C", LangAr},
		{"en", "ar", "ar_EG.UTF-8", LangEn},
		{"", "ar", "C", LangAr},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, ChooseLang(tt.flag, tt.answers, tt.locale), tt)
	}
}
