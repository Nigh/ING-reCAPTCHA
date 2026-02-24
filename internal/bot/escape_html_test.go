package bot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeHTML(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"less-than", "<", "&lt;"},
		{"greater-than", ">", "&gt;"},
		{"ampersand", "&", "&amp;"},
		{"combined tags", "<b>&</b>", "&lt;b&gt;&amp;&lt;/b&gt;"},
		{"empty string", "", ""},
		{"no special chars", "hello world", "hello world"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, escapeHTML(tc.input))
		})
	}
}
