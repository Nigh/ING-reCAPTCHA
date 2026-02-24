package i18n

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_Success(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		tr, err := New(lang)
		require.NoError(t, err, "New(%q) should succeed", lang)
		assert.NotNil(t, tr)
	}
}

func TestNew_UnknownDefaultLang(t *testing.T) {
	_, err := New("xx")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "xx")
}

func TestT_ExactMatch(t *testing.T) {
	tr, err := New("en")
	require.NoError(t, err)

	got := tr.T("en", "err_permission_denied")
	assert.Equal(t, "❌ Permission denied", got)
}

func TestT_FallbackToDefault(t *testing.T) {
	tr, err := New("en")
	require.NoError(t, err)

	// "fr" is not a supported language; should fall back to "en"
	got := tr.T("fr", "err_permission_denied")
	assert.Equal(t, "❌ Permission denied", got)
}

func TestT_MissingKey(t *testing.T) {
	tr, err := New("en")
	require.NoError(t, err)

	const missingKey = "this_key_does_not_exist"
	got := tr.T("en", missingKey)
	assert.Equal(t, missingKey, got)
}

func TestT_WithArgs(t *testing.T) {
	tr, err := New("en")
	require.NoError(t, err)

	// "cmd_help_image_count" = "• Image count: %d\n"
	got := tr.T("en", "cmd_help_image_count", 5)
	assert.Equal(t, "• Image count: 5\n", got)
}

func TestLanguageFallbacks(t *testing.T) {
	t.Run("zh-TW expands to zh-tw, zh, en", func(t *testing.T) {
		got := languageFallbacks("zh-TW", "en")
		assert.Equal(t, []string{"zh-tw", "zh", "en"}, got)
	})

	t.Run("deduplication when lang equals defaultLang", func(t *testing.T) {
		got := languageFallbacks("en", "en")
		assert.Equal(t, []string{"en"}, got)
	})

	t.Run("simple lang with different default", func(t *testing.T) {
		got := languageFallbacks("zh", "en")
		assert.Equal(t, []string{"zh", "en"}, got)
	})
}

func TestNormalizeLang(t *testing.T) {
	assert.Equal(t, "en", normalizeLang("EN"))
	assert.Equal(t, "zh-tw", normalizeLang("ZH-TW"))
	assert.Equal(t, "en", normalizeLang("  en  "))
	assert.Equal(t, "", normalizeLang(""))
	assert.Equal(t, "", normalizeLang("   "))
}

func TestIsSupported(t *testing.T) {
	tr, err := New("en")
	require.NoError(t, err)

	assert.True(t, tr.IsSupported("en"))
	assert.True(t, tr.IsSupported("zh"))
	assert.False(t, tr.IsSupported("xx"))
}

func TestGetSupportedLanguages(t *testing.T) {
	tr, err := New("en")
	require.NoError(t, err)

	langs := tr.GetSupportedLanguages()
	assert.True(t, sort.StringsAreSorted(langs), "languages should be sorted")
	assert.Contains(t, langs, "en")
	assert.Contains(t, langs, "zh")
}
