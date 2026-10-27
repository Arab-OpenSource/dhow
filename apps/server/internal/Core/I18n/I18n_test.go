package i18n_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	i18n "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/I18n"
)

func TestTranslate(t *testing.T) {
	tests := []struct {
		name   string
		locale i18n.Locale
		key    string
		want   string
	}{
		{"arabic", i18n.Ar, "errors.notFound", "المورد المطلوب مش موجود."},
		{"english", i18n.En, "errors.notFound", "The requested resource does not exist."},
		{"unknown locale uses arabic", i18n.Locale("fr"), "errors.notFound", "المورد المطلوب مش موجود."},
		{"unknown key returns the key", i18n.Ar, "no.such.key", "no.such.key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, i18n.Translate(tt.locale, tt.key))
		})
	}
}

func TestCatalogsCoverTheSameKeys(t *testing.T) {
	keys := []string{
		"errors.internal", "errors.notFound", "errors.methodNotAllowed", "errors.badRequest",
		"errors.validationFailed", "errors.payloadTooLarge", "errors.serviceUnavailable",
		"system.health.live", "system.health.ready", "system.meta.loaded",
	}
	for _, k := range keys {
		assert.NotEqual(t, k, i18n.Translate(i18n.Ar, k), "missing Arabic message for %s", k)
		assert.NotEqual(t, k, i18n.Translate(i18n.En, k), "missing English message for %s", k)
	}
}

func TestLocaleContext(t *testing.T) {
	assert.Equal(t, i18n.Ar, i18n.LocaleFrom(context.Background()))
	assert.Equal(t, i18n.En, i18n.LocaleFrom(i18n.WithLocale(context.Background(), i18n.En)))
}

func TestParse(t *testing.T) {
	tests := []struct {
		header string
		want   i18n.Locale
	}{
		{"", i18n.Ar},
		{"en", i18n.En},
		{"EN-gb", i18n.En},
		{"en;q=0.3, ar;q=0.7", i18n.Ar},
		{"en;q=bad", i18n.En},
		{"fr", i18n.Ar},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			assert.Equal(t, tt.want, i18n.Parse(tt.header))
		})
	}
}
