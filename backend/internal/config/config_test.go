package config

import (
	"strings"
	"testing"
)

// The four knobs that name a village have no default: a binary started
// without them says which are missing, all of them at once.
func TestMissing(t *testing.T) {
	var c Config
	if got := strings.Join(c.Missing(), ","); got != "VILLAGE_NAME,PUBLIC_URL,PUSH_SUBJECT,LANGUAGES,WEATHER_PROVIDER (open-meteo or arso)" {
		t.Fatalf("missing: %s", got)
	}
	c.VillageName, c.PublicURL, c.PushSubject, c.Languages = "x", "x", "x", splitList(" sl, en ")
	if m := c.Missing(); len(m) != 1 || m[0] != "WEATHER_PROVIDER (open-meteo or arso)" {
		t.Fatalf("an empty provider must be named: %v", m)
	}
	c.WeatherProvider = "open-meteo"
	if m := c.Missing(); len(m) != 0 {
		t.Fatalf("missing after set: %v", m)
	}
	if c.WeatherOn() {
		t.Fatal("open-meteo without coords is on")
	}
	c.WeatherCoords = "45.6,14.9"
	if !c.WeatherOn() {
		t.Fatal("open-meteo with coords is off")
	}
	c.WeatherProvider, c.WeatherLocation = "arso", ""
	if c.WeatherOn() {
		t.Fatal("arso without a place is on")
	}
	if c.Default() != "sl" || len(c.Languages) != 2 {
		t.Fatalf("languages: %v", c.Languages)
	}
}
