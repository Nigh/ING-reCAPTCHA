package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeLangCode(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"uppercase", "ZH", "zh"},
		{"hyphen variant", "zh-TW", "zh"},
		{"underscore variant", "en_US", "en"},
		{"trim spaces", "  en  ", "en"},
		{"empty string", "", ""},
		{"already normalized", "en", "en"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, normalizeLangCode(tc.input))
		})
	}
}
