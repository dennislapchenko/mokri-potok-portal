package httpapi

import (
	"regexp"
	"strings"
	"testing"
)

var verbs = regexp.MustCompile(`%(\[\d+\])?[sdv]`)

// TestDictionaries: a translation keeps the key's shape — the same format
// verbs, so a banner never prints %!s(MISSING) on a lock screen, and the same
// leading and trailing spaces, because phrases are glued to names in code.
// A dictionary with an icon in a key is refused too: icons stay in the code.
func TestDictionaries(t *testing.T) {
	if len(dicts) == 0 {
		t.Fatal("no dictionaries embedded")
	}
	for lang, d := range dicts {
		for k, v := range d {
			if strings.TrimLeft(k, " ") == "" || strings.TrimSpace(v) == "" {
				t.Errorf("%s: %q is empty", lang, k)
			}
			if len(verbs.FindAllString(k, -1)) != len(verbs.FindAllString(v, -1)) {
				t.Errorf("%s: %q -> %q: format verbs differ", lang, k, v)
			}
			if lead(k) != lead(v) || trail(k) != trail(v) {
				t.Errorf("%s: %q -> %q: the spaces at the ends differ", lang, k, v)
			}
			if r := []rune(strings.TrimLeft(k, " "))[0]; r > 0x2000 && !strings.ContainsRune("–—…«»", r) {
				t.Errorf("%s: %q starts with a symbol; icons belong in the code", lang, k)
			}
		}
	}
}

func lead(s string) int  { return len(s) - len(strings.TrimLeft(s, " ")) }
func trail(s string) int { return len(s) - len(strings.TrimRight(s, " ")) }
