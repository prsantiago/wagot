package dotenv

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	const input = `
# a comment
PLAIN=value
export EXPORTED=yes
SPACED = padded value
EMPTY=
TRAILING=value # comment
HASH=abc#def
DOUBLE="hello # not a comment\nworld"
ESCAPED="say \"hi\"" # comment
SINGLE='literal \n $HOME'
EQUALS=a=b=c
`
	want := map[string]string{
		"PLAIN":    "value",
		"EXPORTED": "yes",
		"SPACED":   "padded value",
		"EMPTY":    "",
		"TRAILING": "value",
		"HASH":     "abc#def",
		"DOUBLE":   "hello # not a comment\nworld",
		"ESCAPED":  `say "hi"`,
		"SINGLE":   `literal \n $HOME`,
		"EQUALS":   "a=b=c",
	}

	got, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !maps.Equal(got, want) {
		t.Errorf("Parse =\n%q\nwant\n%q", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	tests := map[string]string{
		"no equals":          "JUSTAKEY",
		"empty key":          "=value",
		"invalid key":        "MY-KEY=value",
		"leading digit":      "1KEY=value",
		"unterminated dq":    `KEY="open`,
		"unterminated sq":    `KEY='open`,
		"escaped close only": `KEY="open\"`,
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(input)); err == nil {
				t.Errorf("Parse(%q) succeeded, want error", input)
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Load = %v, want empty", got)
	}
}
