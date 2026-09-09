// Package config reads the process environment once. Every knob has a default
// that works in a container with nothing set; nothing here is a secret.
package config

import "os"

type Config struct {
	Port            string // HTTP listen port
	Bind            string // interface to listen on; empty = all. Dev sets 127.0.0.1 so the macOS firewall stops asking
	DataDir         string // SQLite file + nightly backups live here
	BootstrapCode   string // optional: fixed code for the first steward house; empty = generated and logged
	PushSubject     string // VAPID subject: an https URL or mailto: that identifies this sender to push services
	WeatherLocation string // ARSO location name for the home-screen weather; empty turns the panel off
	PublicURL       string // where the portal answers, used to print invite links
	Debug           bool
}

func Load() Config {
	return Config{
		Port:            envOr("PORT", "8788"),
		Bind:            os.Getenv("BIND"),
		DataDir:         envOr("DATA_DIR", "./data"),
		BootstrapCode:   os.Getenv("POTOK_BOOTSTRAP_CODE"),
		PushSubject:     envOr("PUSH_SUBJECT", "https://vas.mokri-potok.si/"),
		WeatherLocation: envOr("WEATHER_LOCATION", "Kočevje"),
		PublicURL:       envOr("PUBLIC_URL", "https://vas.mokri-potok.si"),
		Debug:           os.Getenv("DEBUG") == "1",
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
