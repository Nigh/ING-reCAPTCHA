package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed locales/*.json
var localesFS embed.FS

// Translator provides internationalized string lookup with fallback support.
type Translator struct {
	translations map[string]map[string]string // lang -> key -> text
	defaultLang  string
}

// New creates a Translator by loading all .json files from the embedded locales
// directory. Each file's name (without extension) is used as the language code.
// Returns an error if no locale files are found or if keys are inconsistent
// across locale files.
func New(defaultLang string) (*Translator, error) {
	entries, err := localesFS.ReadDir("locales")
	if err != nil {
		return nil, fmt.Errorf("i18n: reading locales directory: %w", err)
	}

	translations := make(map[string]map[string]string)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}

		data, err := localesFS.ReadFile(filepath.Join("locales", name))
		if err != nil {
			return nil, fmt.Errorf("i18n: reading %s: %w", name, err)
		}

		var msgs map[string]string
		if err := json.Unmarshal(data, &msgs); err != nil {
			return nil, fmt.Errorf("i18n: parsing %s: %w", name, err)
		}

		lang := strings.TrimSuffix(name, ".json")
		translations[lang] = msgs
	}

	if len(translations) == 0 {
		return nil, fmt.Errorf("i18n: no locale files found")
	}

	defaultLang = normalizeLang(defaultLang)
	if _, ok := translations[defaultLang]; !ok {
		if idx := strings.IndexAny(defaultLang, "-_"); idx > 0 {
			base := defaultLang[:idx]
			if _, ok := translations[base]; ok {
				defaultLang = base
			}
		}
	}
	if _, ok := translations[defaultLang]; !ok {
		return nil, fmt.Errorf("i18n: default language %q not found in locale files", defaultLang)
	}

	// Validate that all locale files have the same set of keys.
	if err := validateKeys(translations); err != nil {
		return nil, err
	}

	return &Translator{
		translations: translations,
		defaultLang:  defaultLang,
	}, nil
}

// T returns the translated string for the given language and key.
// Falls back to the default language if the key is missing in the requested
// language. Returns the key itself if no translation is found at all.
// Optional args are passed to fmt.Sprintf for parameter substitution.
func (t *Translator) T(lang, key string, args ...interface{}) string {
	for _, candidate := range languageFallbacks(lang, t.defaultLang) {
		if msgs, ok := t.translations[candidate]; ok {
			if text, ok := msgs[key]; ok {
				return applyArgs(text, args)
			}
		}
	}

	return key
}

// IsSupported reports whether the given language code has loaded translations.
func (t *Translator) IsSupported(lang string) bool {
	_, ok := t.translations[normalizeLang(lang)]
	return ok
}

// GetSupportedLanguages returns a sorted list of all loaded language codes.
func (t *Translator) GetSupportedLanguages() []string {
	langs := make([]string, 0, len(t.translations))
	for lang := range t.translations {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	return langs
}

// applyArgs formats text with args if any are provided.
func applyArgs(text string, args []interface{}) string {
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

func normalizeLang(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "" {
		return ""
	}
	return lang
}

func languageFallbacks(lang, defaultLang string) []string {
	seen := make(map[string]bool)
	var candidates []string

	lang = normalizeLang(lang)
	if lang != "" {
		candidates = append(candidates, lang)
		seen[lang] = true

		if idx := strings.IndexAny(lang, "-_"); idx > 0 {
			base := lang[:idx]
			if base != "" && !seen[base] {
				candidates = append(candidates, base)
				seen[base] = true
			}
		}
	}

	defaultLang = normalizeLang(defaultLang)
	if defaultLang != "" && !seen[defaultLang] {
		candidates = append(candidates, defaultLang)
	}

	return candidates
}

// validateKeys checks that every locale file contains exactly the same set of
// keys. Returns a descriptive error on the first mismatch found.
func validateKeys(translations map[string]map[string]string) error {
	// Pick a reference language (first in sorted order for determinism).
	langs := make([]string, 0, len(translations))
	for lang := range translations {
		langs = append(langs, lang)
	}
	sort.Strings(langs)

	refLang := langs[0]
	refKeys := translations[refLang]

	for _, lang := range langs[1:] {
		msgs := translations[lang]

		for key := range refKeys {
			if _, ok := msgs[key]; !ok {
				return fmt.Errorf("i18n: language %q is missing key %q (present in %q)", lang, key, refLang)
			}
		}
		for key := range msgs {
			if _, ok := refKeys[key]; !ok {
				return fmt.Errorf("i18n: language %q has extra key %q (not in %q)", lang, key, refLang)
			}
		}
	}

	return nil
}
