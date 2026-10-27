package installer

import (
	"fmt"
	"strings"
)

type Key string

type Messages struct {
	Lang  string
	table map[Key]string
}

func NewMessages(lang string) Messages {
	if lang == LangAr {
		return Messages{Lang: LangAr, table: arMessages}
	}
	return Messages{Lang: LangEn, table: enMessages}
}

func (m Messages) T(k Key, args ...any) string {
	s, ok := m.table[k]
	if !ok {
		s, ok = enMessages[k]
	}
	if !ok {
		s = string(k)
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
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

func MessageKeys(lang string) []Key {
	table := enMessages
	if lang == LangAr {
		table = arMessages
	}
	keys := make([]Key, 0, len(table))
	for k := range table {
		keys = append(keys, k)
	}
	return keys
}
