// Package config reads the process environment once. Every knob has a default
// that works in a container with nothing set; nothing here is a secret.
package config

import (
	"os"
	"strings"
)

type Config struct {
	Port            string   // HTTP listen port
	DataDir         string   // SQLite file + nightly backups live here
	CORSOrigins     []string // exact origins allowed to call the API from a browser
	BootstrapCode   string   // optional: fixed code for the first steward house; empty = generated and logged
	PushSubject     string   // VAPID subject: an https URL or mailto: that identifies this sender to push services
	WeatherLocation string   // ARSO location name for the home-screen weather; empty turns the panel off
	Debug           bool
}

func Load() Config {
	return Config{
		Port:            envOr("PORT", "8788"),
		DataDir:         envOr("DATA_DIR", "./data"),
		CORSOrigins:     splitCSV(envOr("CORS_ORIGINS", "http://localhost:5173,https://dennislapchenko.github.io")),
		BootstrapCode:   os.Getenv("POTOK_BOOTSTRAP_CODE"),
		PushSubject:     envOr("PUSH_SUBJECT", "https://vas.mokri-potok.si/"),
		WeatherLocation: envOr("WEATHER_LOCATION", "Kočevje"),
		Debug:           os.Getenv("DEBUG") == "1",
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
