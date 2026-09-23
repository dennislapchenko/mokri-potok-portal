package httpapi

import "testing"

// TestWeatherIcons: both providers land on the same small set of icon keys,
// and Open-Meteo's texts are dictionary keys.
func TestWeatherIcons(t *testing.T) {
	for _, c := range []struct {
		code int
		day  bool
		icon string
		text string
	}{
		{0, true, icClear, "Clear sky"}, {0, false, icClearNight, "Clear sky"}, {2, true, icPartlyCloudy, "Partly cloudy"},
		{45, true, icFog, "Fog"}, {55, true, icDrizzle, "Drizzle"}, {63, true, icRain, "Rain"}, {73, true, icSnow, "Snow"},
		{81, true, icRain, "Rain showers"}, {95, true, icThunder, "Thunderstorm"},
	} {
		if icon, text := wmo(c.code, c.day); icon != c.icon || text != c.text {
			t.Errorf("wmo(%d, %v) = %q %q", c.code, c.day, icon, text)
		}
		if _, ok := dicts["sl"][c.text]; !ok {
			t.Errorf("sl.json lacks %q", c.text)
		}
	}
	for name, want := range map[string]string{
		"clear_day": icClear, "clear_night": icClearNight, "partCloudy_day": icPartlyCloudy, "overcast_lightRA_day": icRain,
		"prevCloudy_modRASN_night": icSnow, "prevCloudy_day": icMostlyCloudy, "modCloudy_lightDZ_day": icDrizzle, "overcast_TS_day": icThunder, "FG_day": icFog, "slightCloudy_day": icMostlyClear, "": "",
	} {
		if got := arsoIcon(name); got != want {
			t.Errorf("arsoIcon(%q) = %q, want %q", name, got, want)
		}
	}
}
