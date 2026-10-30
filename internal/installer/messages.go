package installer

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

const (
	LangEn = "en"
	LangAr = "ar"
)

type Key string

type Messages struct {
	Lang     string
	table    map[Key]string
	fallback map[Key]string
}

func NewMessages(lang string, en, ar map[Key]string) Messages {
	english := merged(engineEn, en)
	if lang == LangAr {
		return Messages{Lang: LangAr, table: merged(engineAr, ar), fallback: english}
	}
	return Messages{Lang: LangEn, table: english, fallback: english}
}

func merged(base, app map[Key]string) map[Key]string {
	out := make(map[Key]string, len(base)+len(app))
	maps.Copy(out, base)
	maps.Copy(out, app)
	return out
}

func (m Messages) T(k Key, args ...any) string {
	s, ok := m.table[k]
	if !ok {
		s, ok = m.fallback[k]
	}
	if !ok {
		s = string(k)
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

func (m Messages) YesNo(v bool) string {
	if v {
		return m.T(KeyYes)
	}
	return m.T(KeyNo)
}

func ChooseLang(flagLang, answersLang, locale string) string {
	for _, l := range []string{flagLang, answersLang} {
		if l == LangEn || l == LangAr {
			return l
		}
	}
	l := strings.ToLower(locale)
	utf8 := strings.Contains(l, "utf-8") || strings.Contains(l, "utf8")
	if utf8 && strings.HasPrefix(l, "ar") {
		return LangAr
	}
	return LangEn
}

func Keys(table map[Key]string) []Key {
	return slices.Sorted(maps.Keys(table))
}

func EngineCatalog(lang string) map[Key]string {
	if lang == LangAr {
		return engineAr
	}
	return engineEn
}
