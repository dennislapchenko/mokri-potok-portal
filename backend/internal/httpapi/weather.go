package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Weather for the home screen, from ARSO (the Slovenian environment agency).
//
// The backend fetches and trims it instead of the page framing the agency's
// widget: an iframe would hand ARSO every villager's address and the portal URL
// on a logged-in page, and would need a hole in any future CSP. Here the agency
// sees one server, once every half hour.
//
// Source: https://vreme.arso.gov.si — public data, attribution shown in the UI.

type weatherDay struct {
	Date string `json:"date"`
	Icon string `json:"icon"`
	Text string `json:"text"`
	Min  string `json:"min"`
	Max  string `json:"max"`
	Rain string `json:"rain"`
}

type weatherOut struct {
	Place   string       `json:"place"`
	Now     string       `json:"now"`
	NowIcon string       `json:"now_icon"`
	NowText string       `json:"now_text"`
	Wind    string       `json:"wind"`
	Days    []weatherDay `json:"days"`
	Fetched string       `json:"fetched"`
	Source  string       `json:"source"`
}

type weatherCache struct {
	mu   sync.Mutex
	at   time.Time
	body []byte
}

var wcache weatherCache

// contact is what ARSO sees if anyone looks at their logs.
const contact = "https://vas.mokri-potok.si/"

func (s *Server) weather(w http.ResponseWriter, r *http.Request) {
	loc := s.cfg.WeatherLocation
	if loc == "" {
		writeErr(w, 503, "no weather location configured")
		return
	}
	wcache.mu.Lock()
	fresh := time.Since(wcache.at) < 30*time.Minute && wcache.body != nil
	body := wcache.body
	wcache.mu.Unlock()
	if !fresh {
		out, err := fetchWeather(r.Context(), loc)
		if err != nil {
			if body != nil { // serve the stale copy rather than nothing
				writeRaw(w, body)
				return
			}
			writeErr(w, 502, "weather unavailable")
			return
		}
		body, _ = json.Marshal(out)
		wcache.mu.Lock()
		wcache.at, wcache.body = time.Now(), body
		wcache.mu.Unlock()
	}
	writeRaw(w, body)
}

func writeRaw(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=900")
	w.Write(body)
}

type arsoFeatureSet struct {
	Features []struct {
		Properties struct {
			Title string `json:"title"`
			Days  []struct {
				Date     string              `json:"date"`
				Timeline []map[string]string `json:"timeline"`
			} `json:"days"`
		} `json:"properties"`
	} `json:"features"`
}

func fetchWeather(ctx context.Context, loc string) (*weatherOut, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://vreme.arso.gov.si/api/1.0/location/?location="+url.QueryEscape(loc), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	// Identify the caller. An anonymous poller is the thing a service blocks
	// first, and there is no published rate limit to rely on.
	req.Header.Set("User-Agent", "mokri-potok-portal/1 (village portal; "+contact+")")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Forecast3h  arsoFeatureSet `json:"forecast3h"`
		Forecast24h arsoFeatureSet `json:"forecast24h"`
		Observation arsoFeatureSet `json:"observation"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	out := &weatherOut{Fetched: time.Now().UTC().Format(time.RFC3339), Source: "ARSO"}
	first := func(fs arsoFeatureSet) map[string]string {
		if len(fs.Features) > 0 && len(fs.Features[0].Properties.Days) > 0 && len(fs.Features[0].Properties.Days[0].Timeline) > 0 {
			return fs.Features[0].Properties.Days[0].Timeline[0]
		}
		return nil
	}
	if len(doc.Forecast3h.Features) > 0 {
		out.Place = doc.Forecast3h.Features[0].Properties.Title
	}
	// Now: the observation if there is one, else the nearest 3-hour step.
	now := first(doc.Observation)
	if now == nil || now["t"] == "" {
		now = first(doc.Forecast3h)
	}
	if now != nil {
		out.Now, out.NowIcon, out.NowText = now["t"], now["clouds_icon_wwsyn_icon"], now["clouds_shortText_wwsyn_shortText"]
		if out.NowText == "" {
			out.NowText = now["clouds_shortText"]
		}
		out.Wind = now["ff_shortText"]
	}
	if len(doc.Forecast24h.Features) > 0 {
		for _, d := range doc.Forecast24h.Features[0].Properties.Days {
			if len(d.Timeline) == 0 || len(out.Days) >= 5 {
				break
			}
			tl := d.Timeline[0]
			day := weatherDay{Date: d.Date, Icon: tl["clouds_icon_wwsyn_icon"], Min: tl["tnsyn"], Max: tl["txsyn"], Rain: tl["tp_24h_acc"]}
			day.Text = tl["clouds_shortText_wwsyn_shortText"]
			if day.Text == "" {
				day.Text = tl["clouds_shortText"]
			}
			// Millimetres, but only when it is worth saying.
			if v, err := strconv.ParseFloat(strings.TrimSpace(day.Rain), 64); err != nil || v < 0.1 {
				day.Rain = ""
			}
			out.Days = append(out.Days, day)
		}
	}
	if out.Now == "" && len(out.Days) == 0 {
		return nil, errNoWeather
	}
	return out, nil
}

type weatherErr string

func (e weatherErr) Error() string { return string(e) }

const errNoWeather = weatherErr("arso returned nothing usable")
