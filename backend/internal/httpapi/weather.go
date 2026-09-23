package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Weather for the home screen, from the provider WEATHER_PROVIDER names:
// open-meteo (the default — anywhere on earth from WEATHER_COORDS, no key,
// CC BY 4.0, so the panel names Open-Meteo.com) or arso (the Slovenian
// environment agency, by the place name in WEATHER_LOCATION — this village).
//
// The backend fetches and trims it instead of the page framing a widget: an
// iframe would hand the provider every villager's address and the portal URL
// on a logged-in page, and would need a hole in the CSP. Here the provider
// sees one server, once every half hour.
//
// Both adapters emit the same shape. The icon is one of the keys below, which
// the page turns into a glyph; the text is a dictionary key for Open-Meteo
// (the page translates it) and ARSO's own Slovenian words for ARSO.

// Icon keys. A provider maps its own names onto these; the page knows only these.
const (
	icClear        = "clear"
	icClearNight   = "clear-night"
	icMostlyClear  = "mostly-clear"
	icPartlyCloudy = "partly-cloudy"
	icMostlyCloudy = "mostly-cloudy"
	icOvercast     = "overcast"
	icFog          = "fog"
	icDrizzle      = "drizzle"
	icRain         = "rain"
	icSnow         = "snow"
	icThunder      = "thunder"
)

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

func (s *Server) weather(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.WeatherOn() {
		// Not configured is not down: on a 404 the panel says this village
		// has no forecast, where a 503 would have it say the service did not
		// answer.
		writeErr(w, 404, "no weather configured")
		return
	}
	wcache.mu.Lock()
	fresh := time.Since(wcache.at) < 30*time.Minute && wcache.body != nil
	body := wcache.body
	wcache.mu.Unlock()
	if !fresh {
		out, err := s.fetchWeather(r.Context())
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

// fetchWeather asks the configured provider. PUBLIC_URL rides in the
// User-Agent as the contact: what the provider sees if anyone looks at their
// logs, and an anonymous poller is the thing a service blocks first.
func (s *Server) fetchWeather(ctx context.Context) (*weatherOut, error) {
	if s.cfg.WeatherProvider == "arso" {
		return fetchARSO(ctx, s.cfg.WeatherLocation, s.cfg.PublicURL)
	}
	return fetchOpenMeteo(ctx, s.cfg.WeatherCoords, s.cfg.WeatherLocation, s.cfg.PublicURL)
}

func getJSON(ctx context.Context, u, contact string, into any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "porta-pagi/1 (village portal; "+contact+")")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// ---- Open-Meteo ------------------------------------------------------------
//
// https://open-meteo.com — a forecast for any lat,lon, WMO weather codes.
// Attribution is a licence term (CC BY 4.0): the panel names Open-Meteo.com.

func fetchOpenMeteo(ctx context.Context, coords, place, contact string) (*weatherOut, error) {
	lat, lon, ok := strings.Cut(strings.TrimSpace(coords), ",")
	if !ok {
		return nil, errNoWeather
	}
	q := url.Values{
		"latitude": {strings.TrimSpace(lat)}, "longitude": {strings.TrimSpace(lon)},
		"current":       {"temperature_2m,weather_code,wind_speed_10m,is_day"},
		"daily":         {"weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum"},
		"timezone":      {"auto"},
		"forecast_days": {"5"},
	}
	var doc struct {
		Current struct {
			Temp  float64 `json:"temperature_2m"`
			Code  int     `json:"weather_code"`
			Wind  float64 `json:"wind_speed_10m"`
			IsDay int     `json:"is_day"`
		} `json:"current"`
		Daily struct {
			Time []string  `json:"time"`
			Code []int     `json:"weather_code"`
			Max  []float64 `json:"temperature_2m_max"`
			Min  []float64 `json:"temperature_2m_min"`
			Rain []float64 `json:"precipitation_sum"`
		} `json:"daily"`
	}
	if err := getJSON(ctx, "https://api.open-meteo.com/v1/forecast?"+q.Encode(), contact, &doc); err != nil {
		return nil, err
	}
	if place == "" {
		place = strings.TrimSpace(coords)
	}
	out := &weatherOut{Fetched: time.Now().UTC().Format(time.RFC3339), Source: "Open-Meteo.com", Place: place}
	out.NowIcon, out.NowText = wmo(doc.Current.Code, doc.Current.IsDay == 1)
	out.Now = fmt.Sprintf("%.0f", doc.Current.Temp)
	out.Wind = fmt.Sprintf("%.0f km/h", doc.Current.Wind)
	for i, date := range doc.Daily.Time {
		if i >= len(doc.Daily.Code) || i >= len(doc.Daily.Max) || i >= len(doc.Daily.Min) || len(out.Days) >= 5 {
			break
		}
		day := weatherDay{Date: date, Min: fmt.Sprintf("%.0f", doc.Daily.Min[i]), Max: fmt.Sprintf("%.0f", doc.Daily.Max[i])}
		day.Icon, day.Text = wmo(doc.Daily.Code[i], true)
		if i < len(doc.Daily.Rain) && doc.Daily.Rain[i] >= 0.1 { // millimetres, only when worth saying
			day.Rain = strconv.FormatFloat(doc.Daily.Rain[i], 'f', 1, 64)
		}
		out.Days = append(out.Days, day)
	}
	if len(out.Days) == 0 {
		return nil, errNoWeather
	}
	return out, nil
}

// wmo maps a WMO weather code to an icon key and a dictionary key.
func wmo(code int, day bool) (icon, text string) {
	switch {
	case code == 0 && day:
		return icClear, "Clear sky"
	case code == 0:
		return icClearNight, "Clear sky"
	case code == 1:
		return icMostlyClear, "Mainly clear"
	case code == 2:
		return icPartlyCloudy, "Partly cloudy"
	case code == 3:
		return icOvercast, "Overcast"
	case code == 45 || code == 48:
		return icFog, "Fog"
	case code >= 51 && code <= 57:
		return icDrizzle, "Drizzle"
	case code >= 61 && code <= 67:
		return icRain, "Rain"
	case code >= 71 && code <= 77:
		return icSnow, "Snow"
	case code >= 80 && code <= 82:
		return icRain, "Rain showers"
	case code == 85 || code == 86:
		return icSnow, "Snow showers"
	case code >= 95:
		return icThunder, "Thunderstorm"
	}
	return "", ""
}

// ---- ARSO -------------------------------------------------------------------
//
// https://vreme.arso.gov.si — public data, attribution shown in the UI. The
// texts are ARSO's own Slovenian words and go to the page as they are.

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

func fetchARSO(ctx context.Context, loc, contact string) (*weatherOut, error) {
	var doc struct {
		Forecast3h  arsoFeatureSet `json:"forecast3h"`
		Forecast24h arsoFeatureSet `json:"forecast24h"`
		Observation arsoFeatureSet `json:"observation"`
	}
	if err := getJSON(ctx, "https://vreme.arso.gov.si/api/1.0/location/?location="+url.QueryEscape(loc), contact, &doc); err != nil {
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
		out.Now, out.NowIcon, out.NowText = now["t"], arsoIcon(now["clouds_icon_wwsyn_icon"]), now["clouds_shortText_wwsyn_shortText"]
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
			day := weatherDay{Date: d.Date, Icon: arsoIcon(tl["clouds_icon_wwsyn_icon"]), Min: tl["tnsyn"], Max: tl["txsyn"], Rain: tl["tp_24h_acc"]}
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

// arsoIcon maps ARSO's icon names — cloud cover, then the weather, then
// day/night, e.g. partCloudy_lightRA_day — onto the icon keys.
func arsoIcon(name string) string {
	i := strings.ToLower(name)
	switch {
	case strings.Contains(i, "ts"):
		return icThunder
	case strings.Contains(i, "sn"): // SN and RASN alike
		return icSnow
	case strings.Contains(i, "ra") || strings.Contains(i, "rain") || strings.Contains(i, "dz"):
		return icRain
	case strings.Contains(i, "fg"):
		return icFog
	case strings.Contains(i, "overcast"):
		return icOvercast
	case strings.Contains(i, "mostcloudy"):
		return icMostlyCloudy
	case strings.Contains(i, "partcloudy"):
		return icPartlyCloudy
	case strings.Contains(i, "slightcloudy"):
		return icMostlyClear
	case strings.Contains(i, "clear_night"):
		return icClearNight
	case strings.Contains(i, "clear"):
		return icClear
	}
	return ""
}

type weatherErr string

func (e weatherErr) Error() string { return string(e) }

const errNoWeather = weatherErr("the weather provider returned nothing usable")
