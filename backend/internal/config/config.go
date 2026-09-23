// Package config reads the process environment once. Nothing here is a secret.
// Four knobs name the village and have no default, because a default would
// name one village in every binary: VILLAGE_NAME, PUBLIC_URL, PUSH_SUBJECT,
// LANGUAGES.
package config

import (
	"os"
	"strings"
)

type Config struct {
	Port            string   // HTTP listen port
	Bind            string   // interface to listen on; empty = all. Dev sets 127.0.0.1 so the macOS firewall stops asking
	DataDir         string   // SQLite file + nightly backups live here
	VillageName     string   // what the page and the manifest call this village
	Languages       []string // the village's languages, first is the default: "sl,en"; a phone picks one, a dictionary exists per language
	BootstrapCode   string   // optional: fixed code for the first steward house; empty = generated and logged
	PushSubject     string   // VAPID subject: an https URL or mailto: that identifies this sender to push services
	WeatherLocation string   // ARSO location name for the home-screen weather; empty turns the panel off
	PublicURL       string   // where the portal answers: invite links are printed under it, and it is the contact in outgoing User-Agents
	Debug           bool
}

func Load() Config {
	return Config{
		Port:            envOr("PORT", "8788"),
		Bind:            os.Getenv("BIND"),
		DataDir:         envOr("DATA_DIR", "./data"),
		VillageName:     os.Getenv("VILLAGE_NAME"),
		Languages:       splitList(os.Getenv("LANGUAGES")),
		BootstrapCode:   os.Getenv("PAGI_BOOTSTRAP_CODE"),
		PushSubject:     os.Getenv("PUSH_SUBJECT"),
		WeatherLocation: os.Getenv("WEATHER_LOCATION"),
		PublicURL:       os.Getenv("PUBLIC_URL"),
		Debug:           os.Getenv("DEBUG") == "1",
	}
}

// Default is the village's first language: what a phone gets when it names
// none, or one the village does not speak.
func (c Config) Default() string {
	if len(c.Languages) == 0 {
		return ""
	}
	return c.Languages[0]
}

// Missing names the required variables that are not set, so the binary can
// refuse to start with one line that says what to set instead of serving a
// page called nothing.
func (c Config) Missing() []string {
	var m []string
	for _, kv := range []struct{ k, v string }{{"VILLAGE_NAME", c.VillageName}, {"PUBLIC_URL", c.PublicURL}, {"PUSH_SUBJECT", c.PushSubject}, {"LANGUAGES", c.Default()}} {
		if kv.v == "" {
			m = append(m, kv.k)
		}
	}
	return m
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// splitList reads "sl, en" as ["sl","en"]; empty in, nil out.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
