// Package i18n provides embedded English and Arabic UI strings with
// right-to-left detection for the devhive-tunnel suite.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed locales/*.json
var localesFS embed.FS

var registry = map[string]map[string]string{}

func init() {
	for name, raw := range mustLoadLocales() {
		m := map[string]string{}
		if err := json.Unmarshal(raw, &m); err != nil {
			panic(fmt.Sprintf("i18n: bad locale %q: %v", name, err))
		}
		registry[name] = m
	}
}

func mustLoadLocales() map[string][]byte {
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		panic("i18n: " + err.Error())
	}
	out := map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		b, err := localesFS.ReadFile("locales/" + e.Name())
		if err != nil {
			panic("i18n: " + err.Error())
		}
		out[name] = b
	}
	return out
}

// Set is a resolved lookup set for one language.
type Set struct {
	Lang string
	m    map[string]string
}

// Default is English.
func Default() *Set { return Load("") }

// Load resolves a language code; unknown codes fall back to English.
func Load(lang string) *Set {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if _, ok := registry[lang]; !ok {
		lang = "en"
	}
	return &Set{Lang: lang, m: registry[lang]}
}

// Languages lists all available codes.
func Languages() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

// RTL reports whether the language renders right-to-left.
func (s *Set) RTL() bool { return s.Lang == "ar" }

// T translates a key; unknown keys return the key itself (never blank).
func (s *Set) T(key string, args ...any) string {
	v, ok := s.m[key]
	if !ok {
		if en, e2 := registry["en"][key]; e2 {
			v = en
		} else {
			return key
		}
	}
	if len(args) > 0 {
		v = fmt.Sprintf(v, args...)
	}
	return v
}
