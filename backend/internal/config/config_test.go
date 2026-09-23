package config

import (
	"strings"
	"testing"
)

// The three knobs that name a village have no default: a binary started
// without them says which are missing, all of them at once.
func TestMissing(t *testing.T) {
	var c Config
	if got := strings.Join(c.Missing(), ","); got != "VILLAGE_NAME,PUBLIC_URL,PUSH_SUBJECT" {
		t.Fatalf("missing: %s", got)
	}
	c.VillageName, c.PublicURL, c.PushSubject = "x", "x", "x"
	if m := c.Missing(); len(m) != 0 {
		t.Fatalf("missing after set: %v", m)
	}
}
