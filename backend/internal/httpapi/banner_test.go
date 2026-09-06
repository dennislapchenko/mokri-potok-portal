package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/config"
	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/store"
)

// TestNotificationBanners pins the exact two lines a villager sees on the lock
// screen. Friday 2026-09-04 is "now", so 09-06 is "v nedeljo" / "Sunday".
func TestNotificationBanners(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	fake := &fakeSender{status: map[string]int{}}
	srv := New(st, config.Config{BootstrapCode: "x"})
	srv.send = fake
	srv.now = func() time.Time { return time.Date(2026, 9, 4, 9, 42, 0, 0, time.Local) }
	h := srv.Handler()

	steward := &client{t: t, h: h}
	_, obj, _ := steward.do("POST", "/api/bootstrap", map[string]any{"code": "x", "name": "S"})
	steward.token = obj["token"].(string)
	_, o, _ := steward.do("POST", "/api/houses", map[string]any{"name": "House Žagar"})
	zagar := &client{t: t, h: h}
	_, j, _ := zagar.do("POST", "/api/join", map[string]any{"code": o["invite"].(map[string]any)["code"]})
	zagar.token = j["token"].(string)

	// The steward's phones listen, one in each language.
	for _, l := range []string{"sl", "en"} {
		code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{
			"endpoint": "https://push/" + l, "lang": l, "keys": map[string]any{"p256dh": "p", "auth": "a"}})
		steward.must(204, code, "subscribe "+l)
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   map[string]any
		sl, en [2]string // title, body
	}{
		{"run", "POST", "/api/runs",
			map[string]any{"destination": "Bauhaus Ljubljana", "cutoff_at": "2026-09-06T09:00"},
			[2]string{"🚗 House Žagar gre v Bauhaus Ljubljana", "odhod v nedeljo ob 9:00 — napiši, kaj rabiš"},
			[2]string{"🚗 House Žagar drives to Bauhaus Ljubljana", "leaves Sunday at 9:00 — post what you need"}},
		{"need", "POST", "/api/needs",
			map[string]any{"text": "mleko 2l in kvas"},
			[2]string{"🛒 House Žagar rabi iz trgovine", "mleko 2l in kvas"},
			[2]string{"🛒 House Žagar needs from the shop", "mleko 2l in kvas"}},
		{"offer", "POST", "/api/offers",
			map[string]any{"text": "stara okna, 4 kosi", "tag": "giveaway"},
			[2]string{"🎁 House Žagar podarja", "stara okna, 4 kosi"},
			[2]string{"🎁 House Žagar gives away", "stara okna, 4 kosi"}},
		{"seeds", "POST", "/api/offers",
			map[string]any{"text": "paradižnik, semena", "tag": "seeds"},
			[2]string{"🌱 House Žagar deli semena", "paradižnik, semena"},
			[2]string{"🌱 House Žagar shares seeds", "paradižnik, semena"}},
		{"post", "POST", "/api/posts",
			map[string]any{"body": "wasap denis", "author": "Denis"},
			[2]string{"🍺 Denis · House Žagar v gostilni", "wasap denis"},
			[2]string{"🍺 Denis · House Žagar in the tavern", "wasap denis"}},
		{"work party", "POST", "/api/events",
			map[string]any{"title": "Košnja skupnega travnika", "kind": "work", "starts_at": "2026-09-05T08:30", "place": "pri mlinu"},
			[2]string{"🤝 Košnja skupnega travnika", "jutri ob 8:30 · pri mlinu"},
			[2]string{"🤝 Košnja skupnega travnika", "tomorrow at 8:30 · pri mlinu"}},
		{"away", "POST", "/api/away",
			map[string]any{"from_date": "2026-09-10", "to_date": "2026-09-14", "notes": "kokoši in zalivanje"},
			[2]string{"🕯️ Stražnica", "nekdo je vpisal odsotnost — odpri portal"},
			[2]string{"🕯️ Watchtower", "someone marked an absence — open the portal"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake.mu.Lock()
			fake.sent, fake.payloads = nil, nil
			fake.mu.Unlock()
			code, _, _ := zagar.do(c.method, c.path, c.body)
			if code != 201 {
				t.Fatalf("create: %d", code)
			}
			waitFor(t, 2, fake)
			fake.mu.Lock()
			defer fake.mu.Unlock()
			for i, sub := range fake.sent {
				var p Payload
				if err := json.Unmarshal(fake.payloads[i], &p); err != nil {
					t.Fatal(err)
				}
				want := c.sl
				if sub.Lang == "en" {
					want = c.en
				}
				if p.Title != want[0] || p.Body != want[1] {
					t.Errorf("[%s]\n got title %q\nwant title %q\n got body  %q\nwant body  %q", sub.Lang, p.Title, want[0], p.Body, want[1])
				}
				// Away must never leak dates or notes to a lock screen.
				if p.Kind == "away" {
					for _, leak := range []string{"2026-09-10", "2026-09-14", "kokoši", "Žagar"} {
						if contains([]string{p.Title, p.Body}, leak) || len(p.Body) > 60 {
							t.Errorf("away push leaks %q: %q / %q", leak, p.Title, p.Body)
						}
					}
				}
			}
		})
	}
}
