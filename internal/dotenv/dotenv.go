// Package dotenv reads KEY=VALUE files (".env" files).
//
// Supported syntax:
//
//	# comment
//	KEY=value
//	export KEY=value         # "export" prefix and trailing comments are ignored
//	KEY="double quoted\n"    # Go escape sequences are interpreted
//	KEY='single quoted'      # taken literally
//	KEY=                     # empty value
package dotenv

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

// Load parses the file at path. A missing file is not an error: it returns
// an empty map, so the same binary works where configuration comes only from
// the real environment.
func Load(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return vars, nil
}

// Parse reads KEY=VALUE lines from r. Later keys override earlier ones.
func Parse(r io.Reader) (map[string]string, error) {
	vars := make(map[string]string)
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, raw, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !validKey(key) {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE", n)
		}
		value, err := parseValue(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("line %d: %s: %w", n, key, err)
		}
		vars[key] = value
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return vars, nil
}

func parseValue(raw string) (string, error) {
	switch {
	case strings.HasPrefix(raw, `"`):
		end := closingQuote(raw)
		if end < 0 {
			return "", errors.New("unterminated double quote")
		}
		return strconv.Unquote(raw[:end+1])
	case strings.HasPrefix(raw, "'"):
		end := strings.IndexByte(raw[1:], '\'')
		if end < 0 {
			return "", errors.New("unterminated single quote")
		}
		return raw[1 : end+1], nil
	default:
		// An unquoted value ends at " #", so trailing comments are allowed.
		if i := strings.Index(raw, " #"); i >= 0 {
			raw = raw[:i]
		}
		return strings.TrimSpace(raw), nil
	}
}

// closingQuote returns the index of the unescaped '"' that closes s, or -1.
func closingQuote(s string) int {
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case '"':
			return i
		}
	}
	return -1
}

func validKey(k string) bool {
	if k == "" {
		return false
	}
	for i, c := range k {
		letter := c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
		if !letter && (i == 0 || c < '0' || c > '9') {
			return false
		}
	}
	return true
}
