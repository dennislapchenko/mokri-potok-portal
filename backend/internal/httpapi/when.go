package httpapi

import (
	"fmt"
	"strings"
	"time"
)

// Human dates for notifications. A villager reads "v nedeljo ob 9:00", never
// "2026-09-06T09:00". The browser sends wall-clock local strings from
// <input type="datetime-local">, so nothing here converts a timezone — it only
// needs the village's own date to say "today" and "tomorrow" correctly.

var slWeekday = map[time.Weekday]string{
	time.Monday: "v ponedeljek", time.Tuesday: "v torek", time.Wednesday: "v sredo",
	time.Thursday: "v četrtek", time.Friday: "v petek", time.Saturday: "v soboto", time.Sunday: "v nedeljo",
}

var slMonth = [...]string{"jan.", "feb.", "mar.", "apr.", "maja", "jun.", "jul.", "avg.", "sep.", "okt.", "nov.", "dec."}

// tr picks the Slovenian string unless the phone subscribed in English.
func tr(lang, sl, en string) string {
	if lang == "en" {
		return en
	}
	return sl
}

// parseWhen accepts what the form controls produce: a date, or a date and time.
func parseWhen(s string) (t time.Time, hasTime bool, ok bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true, true
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, false, true
	}
	return time.Time{}, false, false
}

// humanWhen renders a date the way a person says it: today, tomorrow, the
// weekday inside the coming week, otherwise the day and month.
func humanWhen(s, lang string, now time.Time) string {
	t, hasTime, ok := parseWhen(s)
	if !ok {
		return s
	}
	day := func(x time.Time) time.Time { return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, x.Location()) }
	delta := int(day(t).Sub(day(now)).Hours() / 24)

	var out string
	switch {
	case delta == 0:
		out = tr(lang, "danes", "today")
	case delta == 1:
		out = tr(lang, "jutri", "tomorrow")
	case delta == -1:
		out = tr(lang, "včeraj", "yesterday")
	case delta > 1 && delta < 7:
		out = tr(lang, slWeekday[t.Weekday()], t.Weekday().String())
	default:
		out = tr(lang, fmt.Sprintf("%d. %s", t.Day(), slMonth[int(t.Month())-1]), fmt.Sprintf("%d %s", t.Day(), t.Format("Jan")))
	}
	if hasTime {
		out += tr(lang, " ob ", " at ") + fmt.Sprintf("%d:%02d", t.Hour(), t.Minute())
	}
	return out
}

// join glues non-empty parts with a separator — keeps the payload builders flat.
func join(sep string, parts ...string) string {
	kept := parts[:0]
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}
