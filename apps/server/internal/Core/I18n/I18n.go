package i18n

import (
	"context"
	"sort"
	"strconv"
	"strings"
)

type Locale string

const (
	Ar            Locale = "ar"
	En            Locale = "en"
	DefaultLocale        = Ar
)

type localeKey struct{}

func WithLocale(ctx context.Context, l Locale) context.Context {
	return context.WithValue(ctx, localeKey{}, l)
}

func LocaleFrom(ctx context.Context) Locale {
	if l, ok := ctx.Value(localeKey{}).(Locale); ok {
		return l
	}
	return DefaultLocale
}

func Translate(locale Locale, key string) string {
	if msg, ok := catalog(locale)[key]; ok {
		return msg
	}
	if msg, ok := catalog(En)[key]; ok {
		return msg
	}
	return key
}

func Keys(locale Locale) []string {
	m := catalog(locale)
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func catalog(locale Locale) map[string]string {
	if locale == En {
		return enMessages
	}
	return arMessages
}

func Parse(acceptLanguage string) Locale {
	best := DefaultLocale
	bestQ := -1.0
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag, q := splitQuality(part)
		var l Locale
		switch strings.ToLower(strings.SplitN(tag, "-", 2)[0]) {
		case string(Ar):
			l = Ar
		case string(En):
			l = En
		default:
			continue
		}
		if q > bestQ {
			best, bestQ = l, q
		}
	}
	return best
}

func splitQuality(part string) (string, float64) {
	tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
	q := 1.0
	for _, p := range strings.Split(params, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(p), "=")
		if !ok || !strings.EqualFold(name, "q") {
			continue
		}
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
			q = parsed
		}
	}
	return strings.TrimSpace(tag), q
}
