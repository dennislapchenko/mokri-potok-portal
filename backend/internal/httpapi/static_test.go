package httpapi

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestPlaceholderCarriesTokens: the placeholder page committed in web/ must
// carry every {{token}} the real frontend/index.html carries, or the binary's
// tests pass on the built page and CI fails on the bare embed — which is how
// a token went missing once. Skipped where the frontend is not checked out.
func TestPlaceholderCarriesTokens(t *testing.T) {
	real, err := os.ReadFile("../../../frontend/index.html")
	if err != nil {
		t.Skip("no frontend/index.html beside the backend")
	}
	placeholder, err := webFS.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range regexp.MustCompile(`\{\{[A-Z_]+\}\}`).FindAllString(string(real), -1) {
		if !strings.Contains(string(placeholder), tok) {
			t.Errorf("web/index.html lacks %s; the served page fills it, so the placeholder must carry it", tok)
		}
	}
}
