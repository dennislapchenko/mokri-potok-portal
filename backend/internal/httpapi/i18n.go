package httpapi

import (
	"embed"
	"encoding/json"
	"io/fs"
	"strings"
)

// One dictionary per language, i18n/<lang>.json, keyed by the English phrase:
// the same files the frontend imports (vite.config.ts aliases the directory),
// so a notification and the page it opens are translated in one place. There
// is no en.json — English is the key, and a phrase with no entry reads as
// itself. A new language is a new file, nothing else.

//go:embed i18n/*.json
var dictFS embed.FS

var dicts = map[string]map[string]string{}

func init() {
	entries, err := fs.ReadDir(dictFS, "i18n")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		b, err := dictFS.ReadFile("i18n/" + e.Name())
		if err != nil {
			panic(err)
		}
		d := map[string]string{}
		if err := json.Unmarshal(b, &d); err != nil {
			panic("i18n/" + e.Name() + ": " + err.Error())
		}
		dicts[strings.TrimSuffix(e.Name(), ".json")] = d
	}
}

// tr returns the phrase in the phone's language, or the English key itself.
func tr(lang, en string) string {
	if v, ok := dicts[lang][en]; ok {
		return v
	}
	return en
}
