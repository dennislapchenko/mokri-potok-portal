// Package config reads the process environment once. Nothing here is a secret.
// Three knobs name the village and have no default, because a default would
// name one village in every binary: VILLAGE_NAME, PUBLIC_URL, PUSH_SUBJECT.
package config

import "os"

type Config struct {
	Port            string // HTTP listen port
	Bind            string // interface to listen on; empty = all. Dev sets 127.0.0.1 so the macOS firewall stops asking
	DataDir         string // SQLite file + nightly backups live here
	VillageName     string // what the page and the manifest call this village
	BootstrapCode   string // optional: fixed code for the first steward house; empty = generated and logged
	PushSubject     string // VAPID subject: an https URL or mailto: that identifies this sender to push services
	WeatherLocation string // ARSO location name for the home-screen weather; empty turns the panel off
	PublicURL       string // where the portal answers: invite links are printed under it, and it is the contact in outgoing User-Agents
	Debug           bool
}

func Load() Config {
	return Config{
		Port:            envOr("PORT", "8788"),
		Bind:            os.Getenv("BIND"),
		DataDir:         envOr("DATA_DIR", "./data"),
		VillageName:     os.Getenv("VILLAGE_NAME"),
		BootstrapCode:   os.Getenv("PAGI_BOOTSTRAP_CODE"),
		PushSubject:     os.Getenv("PUSH_SUBJECT"),
		WeatherLocation: os.Getenv("WEATHER_LOCATION"),
		PublicURL:       os.Getenv("PUBLIC_URL"),
		Debug:           os.Getenv("DEBUG") == "1",
	}
}

// Missing names the required variables that are not set, so the binary can
// refuse to start with one line that says what to set instead of serving a
// page called nothing.
func (c Config) Missing() []string {
	var m []string
	for _, kv := range []struct{ k, v string }{{"VILLAGE_NAME", c.VillageName}, {"PUBLIC_URL", c.PublicURL}, {"PUSH_SUBJECT", c.PushSubject}} {
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
