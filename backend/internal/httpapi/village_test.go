package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/config"
	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/store"
)

// answerOf: the sign-up row of one house, by name. The list carries every
// house that answered — including the house that called the event, which is
// signed up the moment it creates one — so nothing may read it by position.
func answerOf(t *testing.T, list []map[string]any, name string) map[string]any {
	t.Helper()
	for _, r := range list {
		if r["name"] == name {
			return r
		}
	}
	t.Fatalf("no answer from %s in %v", name, list)
	return nil
}

func newVillage(t *testing.T) (*Server, *fakeSender, *client, *client) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	fake := &fakeSender{status: map[string]int{}}
	srv := New(st, config.Config{BootstrapCode: "x"})
	srv.send = fake
	srv.now = func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.Local) }
	h := srv.Handler()
	steward := &client{t: t, h: h}
	_, obj, _ := steward.do("POST", "/api/bootstrap", map[string]any{"code": "x", "name": "S"})
	steward.token = obj["token"].(string)
	_, o, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Zeleni Volk"})
	other := &client{t: t, h: h}
	_, j, _ := other.do("POST", "/api/join", map[string]any{"code": o["invite"].(map[string]any)["code"]})
	other.token = j["token"].(string)
	return srv, fake, steward, other
}

// TestPairingCode: a logged-in house makes a six-digit code, a second phone of
// the same house uses it once, and the code then dies.
func TestPairingCode(t *testing.T) {
	srv, _, _, volk := newVillage(t)
	code, obj, _ := volk.do("POST", "/api/pair", nil)
	volk.must(201, code, "pair")
	pin := obj["code"].(string)
	if len(pin) != 6 {
		t.Fatalf("want 6 digits, got %q", pin)
	}
	phone2 := &client{t: t, h: srv.Handler()}
	code, j, _ := phone2.do("POST", "/api/join", map[string]any{"code": pin, "device": "phone B"})
	phone2.must(201, code, "join by pairing")
	phone2.token = j["token"].(string)
	_, me, _ := phone2.do("GET", "/api/me", nil)
	if me["name"] != "Zeleni Volk" {
		t.Fatalf("paired into wrong house: %v", me)
	}
	// Single use.
	phone3 := &client{t: t, h: srv.Handler()}
	code, _, _ = phone3.do("POST", "/api/join", map[string]any{"code": pin})
	phone3.must(404, code, "reuse refused")
	// Both phones belong to the house.
	_, _, devs := volk.do("GET", "/api/devices", nil)
	if len(devs) != 2 {
		t.Fatalf("want 2 devices got %d", len(devs))
	}
}

// TestGuessingCostsSomething: wrong codes are throttled per IP and five misses
// drop every live pairing code.
func TestGuessingCostsSomething(t *testing.T) {
	srv, _, _, volk := newVillage(t)
	_, obj, _ := volk.do("POST", "/api/pair", nil)
	pin := obj["code"].(string)
	guess := &client{t: t, h: srv.Handler()}
	seen429 := false
	for i := 0; i < 14; i++ {
		code, _, _ := guess.do("POST", "/api/join", map[string]any{"code": "000000"})
		if code == 429 {
			seen429 = true
			break
		}
	}
	if !seen429 {
		t.Fatal("no rate limit on /api/join")
	}
	// The live code was burned by the misses before the limiter kicked in.
	fresh := &client{t: t, h: srv.Handler()}
	if code, _, _ := fresh.do("POST", "/api/join", map[string]any{"code": pin}); code == 201 {
		t.Fatal("live pairing code survived a guessing run")
	}
}

// TestQuietHours: between 21:00 and 07:00 the village is left alone. No kind
// is exempt since the alarm was removed on 2026-09-06 — a real emergency is a
// phone call, not a notification. The one way through is a phone that asked
// for it (TestQuietHoursOptOut).
func TestQuietHours(t *testing.T) {
	srv, fake, steward, volk := newVillage(t)
	srv.now = func() time.Time { return time.Date(2026, 9, 4, 23, 10, 0, 0, time.Local) }
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push.example/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")

	volk.do("POST", "/api/needs", map[string]any{"text": "mleko"})
	volk.do("POST", "/api/events", map[string]any{"title": "Nekaj", "kind": "work", "starts_at": "2026-09-05T09:00"})
	waitFor(t, 0, fake)

	// Daylight: the same event rings.
	srv.now = func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.Local) }
	volk.do("POST", "/api/events", map[string]any{"title": "Drugo", "kind": "work", "starts_at": "2026-09-05T09:00"})
	waitFor(t, 1, fake)
}

// TestToolShed: a house shares a tool, another takes it, the owner is told,
// and returning frees it.
func TestToolShed(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := volk.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push.example/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	volk.must(204, code, "subscribe")

	code, obj, _ := volk.do("POST", "/api/tools", map[string]any{"name": "Motorna žaga", "notes": "gorivo svoje"})
	volk.must(201, code, "create tool")
	id := itoa(obj["id"].(float64))
	waitFor(t, 0, fake) // the sharing house does not hear its own tool

	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("PUT", "/api/tools/"+id, map[string]any{"take": true})
	steward.must(204, code, "take")
	waitFor(t, 1, fake) // only the owner is told
	code, _, _ = volk.do("PUT", "/api/tools/"+id, map[string]any{"take": true})
	volk.must(409, code, "double take")

	_, _, tools := volk.do("GET", "/api/tools", nil)
	if tools[0]["held_by_name"] != "S" {
		t.Fatalf("holder: %v", tools[0])
	}
	code, _, _ = steward.do("PUT", "/api/tools/"+id, map[string]any{"take": false})
	steward.must(204, code, "return")
	_, _, tools = volk.do("GET", "/api/tools", nil)
	if tools[0]["held_by"] != nil {
		t.Fatalf("not returned: %v", tools[0])
	}
	// A house that neither owns nor holds it cannot rename it.
	code, _, _ = steward.do("PUT", "/api/tools/"+id, map[string]any{"name": "x"})
	if code != 204 { // the steward may, being a steward
		t.Fatalf("steward edit: %d", code)
	}
}

// TestWorkBeeSignup: any house can sign up, the caller of the work bee hears
// about it, and the count comes back on the event.
func TestWorkBeeSignup(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := volk.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push.example/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	volk.must(204, code, "subscribe")
	code, obj, _ := volk.do("POST", "/api/events", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-06T08:00"})
	volk.must(201, code, "event")
	id := itoa(obj["id"].(float64))

	// The house that calls a work bee is at it: no separate tap needed.
	_, _, evs := volk.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 1 || evs[0]["mine"] != "yes" {
		t.Fatalf("caller not signed up: %v", evs[0])
	}

	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "yes"})
	steward.must(204, code, "signup")
	waitFor(t, 1, fake) // the house that called it hears

	_, _, evs = volk.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 2 {
		t.Fatalf("signups: %v", evs[0])
	}
	// `mine` is this house's own answer, or null when it has not answered.
	_, _, evs = steward.do("GET", "/api/events", nil)
	if evs[0]["mine"] != "yes" {
		t.Fatalf("mine answer: %v", evs[0])
	}
	code, _, _ = steward.do("DELETE", "/api/events/"+id+"/signup", nil)
	steward.must(204, code, "sign off")
	_, _, evs = volk.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 1 {
		t.Fatalf("still signed up: %v", evs[0])
	}
	// Signing off is an answer a caller may give too.
	code, _, _ = volk.do("DELETE", "/api/events/"+id+"/signup", nil)
	volk.must(204, code, "the caller signs off")
	_, _, evs = volk.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 0 || evs[0]["mine"] != nil {
		t.Fatalf("caller cannot take it back: %v", evs[0])
	}
}

// TestToolReminder: a tool out for two days nags its holder once a day, the
// owner hears nothing, and returning it clears the clock.
func TestToolReminder(t *testing.T) {
	srv, fake, steward, volk := newVillage(t)
	for _, c := range []struct {
		c  *client
		ep string
	}{{steward, "https://push.example/s"}, {volk, "https://push.example/z"}} {
		code, _, _ := c.c.do("POST", "/api/push/subscribe", map[string]any{"endpoint": c.ep, "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
		c.c.must(204, code, "subscribe")
	}
	_, obj, _ := volk.do("POST", "/api/tools", map[string]any{"name": "Lestev"})
	id := itoa(obj["id"].(float64))
	waitFor(t, 1, fake) // the steward hears about the new tool
	steward.do("PUT", "/api/tools/"+id, map[string]any{"take": true})
	waitFor(t, 2, fake) // the owner hears it was taken

	// Fresh loan: no reminder yet.
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	if n, _ := srv.remindTools(context.Background()); n != 0 {
		t.Fatalf("reminded a fresh loan: %d", n)
	}
	// Two days later: exactly one reminder, to the holder.
	srv.st.Exec(context.Background(), `UPDATE tools SET held_since=date('now','-2 day')`)
	if n, _ := srv.remindTools(context.Background()); n != 1 {
		t.Fatalf("want 1 reminder got %d", n)
	}
	waitFor(t, 1, fake)
	fake.mu.Lock()
	if fake.sent[0].Endpoint != "https://push.example/s" {
		t.Fatalf("reminder went to %s", fake.sent[0].Endpoint)
	}
	fake.mu.Unlock()
	// Same day again: nothing. The next gap equals the two days already held.
	if n, _ := srv.remindTools(context.Background()); n != 0 {
		t.Fatalf("double reminder: %d", n)
	}
	srv.st.Exec(context.Background(), `UPDATE tools SET reminded_at=datetime('now','-1 day')`)
	if n, _ := srv.remindTools(context.Background()); n != 0 {
		t.Fatalf("reminded before the gap doubled: %d", n)
	}
	srv.st.Exec(context.Background(), `UPDATE tools SET reminded_at=datetime('now','-2 day')`)
	if n, _ := srv.remindTools(context.Background()); n != 1 {
		t.Fatalf("want the second reminder after the doubled gap, got %d", n)
	}
	// Returned: no more reminders even if the clock says overdue.
	steward.do("PUT", "/api/tools/"+id, map[string]any{"take": false})
	if n, _ := srv.remindTools(context.Background()); n != 0 {
		t.Fatalf("reminded a returned tool: %d", n)
	}
}

// TestShedPhotosWishes: a tool gets a category and a photo, the list carries a
// flag not the bytes, the photo comes back with its type, and the wishlist
// collects house names, not a score.
func TestShedPhotosWishes(t *testing.T) {
	srv, _, steward, volk := newVillage(t)
	code, obj, _ := volk.do("POST", "/api/tools", map[string]any{"name": "Kosilnica", "category": "garden"})
	volk.must(201, code, "tool")
	id := itoa(obj["id"].(float64))

	// Photo: raw bytes with a type; the steward may not upload to Zeleni Volk's tool? Stewards may.
	req := httptest.NewRequest("PUT", "/api/tools/"+id+"/photo", bytes.NewReader([]byte("\xff\xd8jpegbytes")))
	req.Header.Set("Authorization", "Bearer "+volk.token)
	req.Header.Set("Content-Type", "image/jpeg")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("photo upload: %d %s", rec.Code, rec.Body.String())
	}
	_, _, tools := steward.do("GET", "/api/tools", nil)
	if tools[0]["category"] != "garden" || tools[0]["has_photo"].(float64) != 1 {
		t.Fatalf("list: %v", tools[0])
	}
	if _, ok := tools[0]["photo"]; ok {
		t.Fatal("list leaks the blob")
	}
	req = httptest.NewRequest("GET", "/api/tools/"+id+"/photo", nil)
	req.Header.Set("Authorization", "Bearer "+steward.token)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" || rec.Body.Len() != 11 {
		t.Fatalf("photo get: %d %s %d", rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
	}
	// Unknown category falls back to other.
	code, _, _ = volk.do("PUT", "/api/tools/"+id, map[string]any{"category": "spaceships"})
	volk.must(204, code, "category")
	_, _, tools = volk.do("GET", "/api/tools", nil)
	if tools[0]["category"] != "other" {
		t.Fatalf("category: %v", tools[0]["category"])
	}

	// Wishlist.
	code, obj, _ = volk.do("POST", "/api/wishes", map[string]any{"text": "Cepilec drv"})
	volk.must(201, code, "wish")
	wid := itoa(obj["id"].(float64))
	code, _, _ = steward.do("PUT", "/api/wishes/"+wid, map[string]any{"want": true})
	steward.must(204, code, "want")
	_, _, wishes := steward.do("GET", "/api/wishes", nil)
	var wants []map[string]any
	json.Unmarshal([]byte(wishes[0]["wants"].(string)), &wants)
	if len(wants) != 2 || wishes[0]["mine"].(float64) != 1 {
		t.Fatalf("wants: %v", wishes[0])
	}
	code, _, _ = steward.do("PUT", "/api/wishes/"+wid, map[string]any{"want": false})
	steward.must(204, code, "unwant")
	_, _, wishes = steward.do("GET", "/api/wishes", nil)
	json.Unmarshal([]byte(wishes[0]["wants"].(string)), &wants)
	if len(wants) != 1 {
		t.Fatalf("unwant: %v", wishes[0]["wants"])
	}
	// Export must not choke on the blob.
	code, exp, _ := steward.do("GET", "/api/export", nil)
	steward.must(200, code, "export")
	if _, ok := exp["tools"].([]any)[0].(map[string]any)["photo"]; ok {
		t.Fatal("export carries the blob")
	}
}

// TestSignupIsAnAnswerOnly: a sign-up is one of three words and carries no
// note (2026-09-06 — the comment thread does that job). A body without a state
// is refused rather than read as a yes.
func TestSignupIsAnAnswerOnly(t *testing.T) {
	_, _, steward, volk := newVillage(t)
	_, obj, _ := volk.do("POST", "/api/events", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-06T08:00"})
	id := itoa(obj["id"].(float64))
	code, _, _ := steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"note": "pridem s koso"})
	steward.must(400, code, "a note is not an answer")
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "yes", "note": "pridem s koso"})
	steward.must(204, code, "signup")
	_, _, evs := volk.do("GET", "/api/events", nil)
	var list []map[string]any
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	// Two answers: the house that called it, and the one that answered.
	if len(list) != 2 {
		t.Fatalf("signup_list: %v", evs[0]["signup_list"])
	}
	if _, ok := answerOf(t, list, "S")["note"]; ok {
		t.Fatalf("sign-up still carries a note: %v", list)
	}
}

// TestLivesOnAnothersLand: a house without land of its own is put on the map
// by marking the parcel it lives on. That parcel is not taken from the house
// that holds it, and it never joins the marker's own parcel list — a member
// who rents a hut holds no land, and the map must not say otherwise.
func TestLivesOnAnothersLand(t *testing.T) {
	_, _, steward, volk := newVillage(t)
	_, _, hs := steward.do("GET", "/api/houses", nil)
	var volkID string
	for _, h := range hs {
		if h["name"] == "Zeleni Volk" {
			volkID = itoa(h["id"].(float64))
		}
	}
	code, _, _ := steward.do("PUT", "/api/houses/"+volkID, map[string]any{"parcels": []string{"2494", "2496"}})
	steward.must(204, code, "Zeleni Volk holds two parcels")

	code, obj, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Hiša Modri Ježek", "crest": "🌿"})
	steward.must(201, code, "a house with no land")
	jezek := itoa(obj["id"].(float64))
	code, _, _ = steward.do("PUT", "/api/houses/"+jezek, map[string]any{"parcels": []string{"2494", "2500"}})
	steward.must(204, code, "mark where it lives")

	_, _, hs = volk.do("GET", "/api/houses", nil)
	byName := map[string]map[string]any{}
	for _, h := range hs {
		byName[h["name"].(string)] = h
	}
	// 2494 is Zeleni Volk's: marking it says Modri Ježek lives there, nothing changes hands.
	if p := byName["Zeleni Volk"]["parcels"].([]any); len(p) != 2 {
		t.Fatalf("land taken from the house that holds it: %v", p)
	}
	// 2500 belongs to nobody, so it is Modri Ježek's land like any other assignment.
	if p := byName["Hiša Modri Ježek"]["parcels"].([]any); len(p) != 1 || p[0] != "2500" {
		t.Fatalf("free land not assigned, or a neighbour's counted as its own: %v", p)
	}
	if h := byName["Hiša Modri Ježek"]["homes"].([]any); len(h) != 1 || h[0] != "2494" {
		t.Fatalf("does not live anywhere: %v", h)
	}
	if h := byName["Zeleni Volk"]["homes"].([]any); len(h) != 0 {
		t.Fatalf("a landholder was given a home on its own land: %v", h)
	}

	// Clearing the marks leaves the neighbour's land untouched.
	code, _, _ = steward.do("PUT", "/api/houses/"+jezek, map[string]any{"parcels": []string{}})
	steward.must(204, code, "clear")
	_, _, hs = volk.do("GET", "/api/houses", nil)
	for _, h := range hs {
		if h["name"] == "Hiša Modri Ježek" && len(h["homes"].([]any))+len(h["parcels"].([]any)) != 0 {
			t.Fatalf("marks survived clearing: %v", h)
		}
		if h["name"] == "Zeleni Volk" && len(h["parcels"].([]any)) != 2 {
			t.Fatalf("clearing one house took another's land: %v", h)
		}
	}

	// A house marks where it lives itself — no steward, no waiting.
	jezekC := &client{t: t, h: steward.h}
	_, j, _ := jezekC.do("POST", "/api/join", map[string]any{"code": obj["invite"].(map[string]any)["code"], "device": "koča"})
	jezekC.token = j["token"].(string)
	code, _, _ = jezekC.do("PUT", "/api/houses/"+jezek, map[string]any{"homes": []string{"2496", "3193"}})
	jezekC.must(204, code, "a house says where it lives")
	_, _, hs = jezekC.do("GET", "/api/houses", nil)
	for _, h := range hs {
		if h["name"] != "Hiša Modri Ježek" {
			continue
		}
		// 2496 is Zeleni Volk's, 3193 belongs to nobody yet — both are places to
		// live, and neither is land this house now holds.
		if len(h["homes"].([]any)) != 2 {
			t.Fatalf("marks not kept: %v", h["homes"])
		}
		if len(h["parcels"].([]any)) != 0 {
			t.Fatalf("a house gave itself land by saying it lives there: %v", h["parcels"])
		}
	}
	// And only its own: another villager's house is not its to mark.
	code, _, _ = volk.do("PUT", "/api/houses/"+jezek, map[string]any{"homes": []string{"2494"}})
	volk.must(403, code, "a house marks another house's home")

	// The exit path carries the new table, or a stay ends with it dropped.
	_, exp, _ := steward.do("GET", "/api/export", nil)
	if _, ok := exp["house_homes"]; !ok {
		t.Fatal("export drops where houses live")
	}
}

// TestQuietHoursOptOut: a phone that ticked "ring at night" hears the same
// event that the rest of the village sleeps through, and its house-mates keep
// sleeping — the flag is on the device, not on the house.
func TestQuietHoursOptOut(t *testing.T) {
	srv, fake, steward, volk := newVillage(t)
	// Two phones of the steward's house: the second one asks for night rings.
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push.example/s1", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe phone 1")
	_, pair, _ := steward.do("POST", "/api/pair", nil)
	phone2 := &client{t: t, h: srv.Handler()}
	_, j, _ := phone2.do("POST", "/api/join", map[string]any{"code": pair["code"].(string), "device": "night phone"})
	phone2.token = j["token"].(string)
	code, _, _ = phone2.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push.example/s2", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	phone2.must(204, code, "subscribe phone 2")
	code, _, _ = phone2.do("PUT", "/api/me/device", map[string]any{"quiet_ok": true})
	phone2.must(204, code, "opt out of quiet hours")

	srv.now = func() time.Time { return time.Date(2026, 9, 4, 23, 10, 0, 0, time.Local) }
	volk.do("POST", "/api/events", map[string]any{"title": "Nekaj", "kind": "work", "starts_at": "2026-09-05T09:00"})
	waitFor(t, 1, fake)
	fake.mu.Lock()
	got := []string{}
	for _, sub := range fake.sent {
		got = append(got, sub.Endpoint)
	}
	fake.mu.Unlock()
	if len(got) != 1 || got[0] != "https://push.example/s2" {
		t.Fatalf("night push went to %v, want only the phone that asked", got)
	}

	// The house's own page reports the flag for THIS phone only.
	_, p2, _ := phone2.do("GET", "/api/me/prefs", nil)
	_, p1, _ := steward.do("GET", "/api/me/prefs", nil)
	if p2["quiet_ok"] != true || p1["quiet_ok"] != false {
		t.Fatalf("quiet_ok leaked across phones: %v %v", p1["quiet_ok"], p2["quiet_ok"])
	}
}

// TestHouseWithoutLand: option F, the owner's decision 2026-09-06 — a member
// who lives here without land is an ordinary house with an empty parcel list.
// No new kind, so nothing sorts them apart. Where it lives is a mark on the
// map (house_homes), never a sentence: `about` was dropped on 2026-09-08 and a
// client still sending it is ignored, not refused.
func TestHouseWithoutLand(t *testing.T) {
	_, _, steward, _ := newVillage(t)
	code, obj, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Hiša Modri Ježek", "crest": "🌿"})
	steward.must(201, code, "create a house with no land")
	id := itoa(obj["id"].(float64))

	guest := &client{t: t, h: steward.h}
	_, j, _ := guest.do("POST", "/api/join", map[string]any{"code": obj["invite"].(map[string]any)["code"], "device": "koča"})
	guest.token = j["token"].(string)
	code, _, _ = guest.do("PUT", "/api/houses/"+id, map[string]any{"crest": "🌱", "about": "v koči ob potoku"})
	guest.must(204, code, "an old client still sending about")

	_, _, hs := guest.do("GET", "/api/houses", nil)
	var row map[string]any
	for _, h := range hs {
		if h["name"] == "Hiša Modri Ježek" {
			row = h
		}
	}
	if row == nil || row["kind"] != "house" || row["crest"] != "🌱" {
		t.Fatalf("landless house: %v", row)
	}
	if _, ok := row["about"]; ok {
		t.Fatalf("about is back: %v", row)
	}
	if p, _ := row["parcels"].([]any); len(p) != 0 {
		t.Fatalf("house was given land it never asked for: %v", row["parcels"])
	}

	// The Watchtower treats it like any house.
	code, _, _ = guest.do("POST", "/api/away", map[string]any{"from_date": "2026-10-01", "to_date": "2026-10-08"})
	guest.must(201, code, "away notice from a house without land")
	_, _, aw := steward.do("GET", "/api/away", nil)
	if len(aw) != 1 || aw[0]["house_name"] != "Hiša Modri Ježek" {
		t.Fatalf("away notice: %v", aw)
	}
}

// TestGlobalMute: a steward mutes a kind for the whole village; a villager
// cannot; the house list still shows it as off.
func TestGlobalMute(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push.example/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	code, _, _ = volk.do("PUT", "/api/prefs/global", map[string]any{"off": []string{"needs"}})
	volk.must(403, code, "villager mutes globally")
	code, _, _ = steward.do("PUT", "/api/prefs/global", map[string]any{"off": []string{"needs"}})
	steward.must(204, code, "steward mutes needs")
	_, prefs, _ := volk.do("GET", "/api/me/prefs", nil)
	if g := prefs["global_off"].([]any); len(g) != 1 || g[0] != "needs" {
		t.Fatalf("global_off: %v", prefs)
	}
	volk.do("POST", "/api/needs", map[string]any{"text": "sol"})
	waitFor(t, 0, fake)
	volk.do("POST", "/api/offers", map[string]any{"text": "okna"})
	waitFor(t, 1, fake)
}

// TestMuteReachesNobody: a village-wide mute silences a kind for every house,
// and it records which steward set it and when.
func TestMuteReachesNobody(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push.example/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	steward.do("PUT", "/api/prefs/global", map[string]any{"off": []string{"events"}})
	volk.do("POST", "/api/events", map[string]any{"title": "Nekaj", "kind": "work", "starts_at": "2026-09-05T09:00"})
	waitFor(t, 0, fake)
	_, prefs, _ := volk.do("GET", "/api/me/prefs", nil)
	d := prefs["global_detail"].([]any)[0].(map[string]any)
	if d["kind"] != "events" || d["set_by"] != "S" || d["set_at"] == nil {
		t.Fatalf("global_detail: %v", d)
	}
}

// TestProjects: a project with a takable task, an event linked to it, the
// creator told when a task is taken, done as a state, export complete.
func TestProjects(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := volk.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push.example/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	volk.must(204, code, "subscribe")

	code, obj, _ := volk.do("POST", "/api/projects", map[string]any{"title": "Ograja okoli travnika", "due_at": "2026-10-15", "notes": "300 m"})
	volk.must(201, code, "project")
	pid := itoa(obj["id"].(float64))
	waitFor(t, 0, fake) // author does not hear its own project
	code, obj, _ = volk.do("POST", "/api/projects/"+pid+"/tasks", map[string]any{"title": "Kupiti stebre", "due_at": "2026-09-20"})
	volk.must(201, code, "task")
	tid := itoa(obj["id"].(float64))

	// Steward takes the task; Zeleni Volk (project creator) hears.
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("PUT", "/api/tasks/"+tid, map[string]any{"take": true})
	steward.must(204, code, "take")
	waitFor(t, 1, fake)
	code, _, _ = volk.do("PUT", "/api/tasks/"+tid, map[string]any{"take": true})
	volk.must(409, code, "double take")

	// A third house cannot close it; the holder can, with a note.
	_, o, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Tretja"})
	third := &client{t: t, h: volk.h}
	_, j, _ := third.do("POST", "/api/join", map[string]any{"code": o["invite"].(map[string]any)["code"]})
	third.token = j["token"].(string)
	code, _, _ = third.do("PUT", "/api/tasks/"+tid, map[string]any{"state": "done"})
	third.must(403, code, "stranger closes")
	// A stranger cannot clear the holder; the creator may (things get agreed
	// in real life), and may assign — the assigned house hears.
	_, o2, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Cetrta"})
	fourth := &client{t: t, h: volk.h}
	_, j2, _ := fourth.do("POST", "/api/join", map[string]any{"code": o2["invite"].(map[string]any)["code"]})
	fourth.token = j2["token"].(string)
	code, _, _ = fourth.do("PUT", "/api/tasks/"+tid, map[string]any{"take": false})
	fourth.must(403, code, "stranger clears holder")
	code, _, _ = fourth.do("PUT", "/api/tasks/"+tid, map[string]any{"assigned_to": 1})
	fourth.must(403, code, "stranger assigns")
	code, _, _ = fourth.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push.example/4", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	fourth.must(204, code, "subscribe 4th")
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	_, me4, _ := fourth.do("GET", "/api/me", nil)
	code, _, _ = volk.do("PUT", "/api/tasks/"+tid, map[string]any{"assigned_to": me4["id"]})
	volk.must(204, code, "creator assigns")
	waitFor(t, 1, fake)
	code, _, _ = volk.do("PUT", "/api/tasks/"+tid, map[string]any{"take": false})
	volk.must(204, code, "creator clears")
	code, _, _ = steward.do("PUT", "/api/tasks/"+tid, map[string]any{"take": true})
	steward.must(204, code, "steward takes again")
	code, _, _ = steward.do("PUT", "/api/tasks/"+tid, map[string]any{"state": "done", "closing_note": "kupljeno pri Bauhausu"})
	steward.must(204, code, "holder closes")
	code, _, _ = volk.do("PUT", "/api/tasks/"+tid, map[string]any{"take": true})
	volk.must(409, code, "take a done task")

	// Event linked to the task inherits the project.
	code, _, _ = volk.do("POST", "/api/events", map[string]any{"title": "Postavljanje", "kind": "work", "starts_at": "2026-09-27T09:00", "task_id": json.Number(tid)})
	volk.must(201, code, "event on task")
	_, _, evs := volk.do("GET", "/api/events", nil)
	if evs[0]["project_title"] != "Ograja okoli travnika" || evs[0]["task_title"] != "Kupiti stebre" {
		t.Fatalf("event link: %v", evs[0])
	}
	code, p, _ := third.do("GET", "/api/projects/"+pid, nil)
	third.must(200, code, "get project")
	tasks := p["tasks"].([]any)
	if tasks[0].(map[string]any)["state"] != "done" || tasks[0].(map[string]any)["closing_note"] != "kupljeno pri Bauhausu" || len(p["events"].([]any)) != 1 {
		t.Fatalf("project page: %v", p)
	}
	_, _, list := third.do("GET", "/api/projects", nil)
	if list[0]["tasks"].(float64) != 1 || list[0]["tasks_done"].(float64) != 1 {
		t.Fatalf("list counts: %v", list[0])
	}
	// Done is a state; a stranger cannot set it, the creator can, and reopen.
	code, _, _ = third.do("PUT", "/api/projects/"+pid, map[string]any{"state": "done"})
	third.must(403, code, "stranger finishes project")
	code, _, _ = volk.do("PUT", "/api/projects/"+pid, map[string]any{"state": "done"})
	volk.must(204, code, "finish project")
	code, _, _ = volk.do("PUT", "/api/projects/"+pid, map[string]any{"state": "open"})
	volk.must(204, code, "reopen")
	_, exp, _ := steward.do("GET", "/api/export", nil)
	for _, k := range []string{"projects", "project_tasks", "camp_takings"} {
		if _, ok := exp[k]; !ok {
			t.Fatalf("export lacks %s", k)
		}
	}
}

// TestCamp: a camper arrives (village hears), a house claims the money
// (village hears), hands it over; a tick on arrival lands straight in handed.
func TestCamp(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push.example/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	code, obj, _ := volk.do("POST", "/api/camp", map[string]any{"notes": "siv kamper"})
	volk.must(201, code, "arrived")
	id := itoa(obj["id"].(float64))
	waitFor(t, 1, fake)
	fake.mu.Lock()
	var p Payload
	json.Unmarshal(fake.payloads[0], &p)
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	if p.Kind != "camp" || p.Title != "🏕️ Zeleni Volk: kamper je prišel" || p.Body != "siv kamper" {
		t.Fatalf("arrival push: %+v", p)
	}
	// Steward cannot hand over what nobody holds; steward claims; Zeleni Volk hears... steward is the only phone here.
	code, _, _ = steward.do("PUT", "/api/camp/"+id, map[string]any{"state": "handed"})
	steward.must(409, code, "hand over unclaimed")
	code, _, _ = steward.do("PUT", "/api/camp/"+id, map[string]any{"claim": true, "notes": "2 noči"})
	steward.must(204, code, "claim")
	_, _, rows := volk.do("GET", "/api/camp", nil)
	if rows[0]["state"] != "held" || rows[0]["held_by_name"] != "S" || rows[0]["notes"] != "siv kamper" {
		t.Fatalf("after claim (note must not overwrite): %v", rows[0])
	}
	code, _, _ = volk.do("PUT", "/api/camp/"+id, map[string]any{"claim": true})
	volk.must(409, code, "double claim")
	code, _, _ = volk.do("PUT", "/api/camp/"+id, map[string]any{"state": "handed"})
	volk.must(403, code, "non-holder hands over")
	code, _, _ = steward.do("PUT", "/api/camp/"+id, map[string]any{"state": "handed"})
	steward.must(204, code, "holder hands over")
	// Tick "I have the money": straight to handed, holder = noticer.
	code, _, _ = volk.do("POST", "/api/camp", map[string]any{"notes": "NL družina", "have_money": true})
	volk.must(201, code, "arrived with money")
	_, _, rows = volk.do("GET", "/api/camp", nil)
	var withMoney map[string]any
	for _, r := range rows {
		if r["notes"] == "NL družina" {
			withMoney = r
		}
	}
	if withMoney["state"] != "handed" || withMoney["held_by_name"] != "Zeleni Volk" || withMoney["handed_at"] == nil {
		t.Fatalf("have_money row: %v", withMoney)
	}
	if _, ok := rows[0]["amount_cents"]; ok {
		t.Fatal("camp stores an amount")
	}
}

// TestRsvpAndComments: three answers not one, a moved date marks answers stale,
// any house edits an event, comments thread one level and ring the caller.
func TestRsvpAndComments(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := volk.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push.example/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	volk.must(204, code, "subscribe")
	code, obj, _ := volk.do("POST", "/api/events", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-20T08:00"})
	volk.must(201, code, "event")
	id := itoa(obj["id"].(float64))

	// "not coming" is an answer, not silence: it is stored and it is not a yes.
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "no"})
	steward.must(204, code, "rsvp no")
	_, _, evs := volk.do("GET", "/api/events", nil)
	var list []map[string]any
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	// One yes stands: the house that called it. The steward's no is not one.
	if evs[0]["signups"].(float64) != 1 || answerOf(t, list, "S")["state"] != "no" {
		t.Fatalf("no-answer counted as coming: %v", evs[0])
	}
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "maybe"})
	steward.must(204, code, "rsvp maybe")
	_, _, evs = volk.do("GET", "/api/events", nil)
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if s := answerOf(t, list, "S"); evs[0]["signups"].(float64) != 1 || s["state"] != "maybe" || s["stale"].(float64) != 0 {
		t.Fatalf("maybe folded into coming: %v %v", evs[0], list)
	}

	// Any house edits; moving the time marks earlier answers stale.
	code, _, _ = steward.do("PUT", "/api/events/"+id, map[string]any{"notes": "prinesite grablje"})
	steward.must(204, code, "any house edits")
	_, _, evs = volk.do("GET", "/api/events", nil)
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if evs[0]["edited_by_name"] != "S" || answerOf(t, list, "S")["stale"].(float64) != 0 {
		t.Fatalf("note edit made answers stale: %v", evs[0])
	}
	// The no and the maybe each rang the caller, in goroutines: let both land
	// before the sender is cleared, or a late one is counted as the move's.
	waitFor(t, 2, fake)
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("PUT", "/api/events/"+id, map[string]any{"starts_at": "2026-09-21T08:00"})
	steward.must(204, code, "move the date")
	waitFor(t, 1, fake) // the caller answered yes by calling it, so it hears too
	_, _, evs = volk.do("GET", "/api/events", nil)
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if answerOf(t, list, "S")["stale"].(float64) != 1 || answerOf(t, list, "Zeleni Volk")["stale"].(float64) != 1 {
		t.Fatalf("answer survived a moved date as current: %v", list)
	}

	// Comments: one reply level, a reply to a reply hangs off the root.
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, c1, _ := steward.do("POST", "/api/threads/event/"+id, map[string]any{"body": "Kdaj točno?", "author": "Ana"})
	steward.must(201, code, "comment")
	waitFor(t, 1, fake) // the house that called it hears
	rootID := itoa(c1["id"].(float64))
	_, c2, _ := volk.do("POST", "/api/threads/event/"+id, map[string]any{"body": "Ob osmih", "parent_id": c1["id"]})
	_, c3, _ := steward.do("POST", "/api/threads/event/"+id, map[string]any{"body": "Prav", "parent_id": c2["id"]})
	_, _, cs := volk.do("GET", "/api/threads/event/"+id, nil)
	if len(cs) != 3 {
		t.Fatalf("want 3 comments got %d", len(cs))
	}
	for _, c := range cs {
		if c["id"].(float64) == c3["id"].(float64) && itoa(c["parent_id"].(float64)) != rootID {
			t.Fatalf("reply to a reply did not flatten: %v", c)
		}
	}
	_, _, evs = volk.do("GET", "/api/events", nil)
	if evs[0]["comments"].(float64) != 3 {
		t.Fatalf("comment count: %v", evs[0]["comments"])
	}
	// Only the author or a steward deletes a comment.
	code, _, _ = volk.do("DELETE", "/api/comments/"+rootID, nil)
	volk.must(403, code, "stranger deletes a comment")
	code, _, _ = steward.do("DELETE", "/api/comments/"+rootID, nil)
	steward.must(204, code, "author deletes")
	_, _, cs = volk.do("GET", "/api/threads/event/"+id, nil)
	if len(cs) != 0 {
		t.Fatalf("cascade left %d comments", len(cs))
	}
}

// TestWishOptionsAndThread: the same thread on a wish, plus options anyone can
// add. An option is a finding, never a vote — nothing counts or ranks them.
func TestWishOptionsAndThread(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := volk.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push.example/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	volk.must(204, code, "subscribe")
	code, obj, _ := volk.do("POST", "/api/wishes", map[string]any{"text": "Cepilec drv"})
	volk.must(201, code, "wish")
	wid := itoa(obj["id"].(float64))

	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("POST", "/api/wishes/"+wid+"/options", map[string]any{"text": "Vevor 7t, 180 EUR", "url": "example.com/vevor"})
	steward.must(201, code, "option")
	waitFor(t, 1, fake)
	code, _, _ = steward.do("POST", "/api/wishes/"+wid+"/options", map[string]any{"text": ""})
	steward.must(400, code, "empty option")

	_, _, wishes := volk.do("GET", "/api/wishes", nil)
	var opts []map[string]any
	json.Unmarshal([]byte(wishes[0]["options"].(string)), &opts)
	if len(opts) != 1 || opts[0]["url"] != "https://example.com/vevor" || opts[0]["name"] != "S" {
		t.Fatalf("options: %v", wishes[0]["options"])
	}
	code, c1, _ := steward.do("POST", "/api/threads/wish/"+wid, map[string]any{"body": "Videl sem cenejšega"})
	steward.must(201, code, "wish comment")
	_, c2, _ := volk.do("POST", "/api/threads/wish/"+wid, map[string]any{"body": "Kje?", "parent_id": c1["id"]})
	_, c3, _ := steward.do("POST", "/api/threads/wish/"+wid, map[string]any{"body": "V Kočevju", "parent_id": c2["id"]})
	_, _, cs := volk.do("GET", "/api/threads/wish/"+wid, nil)
	if len(cs) != 3 {
		t.Fatalf("want 3 got %d", len(cs))
	}
	for _, c := range cs {
		if c["id"].(float64) == c3["id"].(float64) && c["parent_id"].(float64) != c1["id"].(float64) {
			t.Fatalf("reply to a reply did not flatten: %v", c)
		}
	}
	_, _, wishes = volk.do("GET", "/api/wishes", nil)
	if wishes[0]["comments"].(float64) != 3 {
		t.Fatalf("comment count: %v", wishes[0]["comments"])
	}
	code, _, _ = steward.do("POST", "/api/threads/event/"+wid, map[string]any{"body": "x"})
	steward.must(404, code, "comment on a missing event")
	code, _, _ = steward.do("POST", "/api/threads/nonsense/1", map[string]any{"body": "x"})
	steward.must(400, code, "unknown subject")
	code, _, _ = volk.do("DELETE", "/api/options/"+itoa(opts[0]["id"].(float64)), nil)
	volk.must(403, code, "stranger removes an option")
	code, _, _ = steward.do("DELETE", "/api/options/"+itoa(opts[0]["id"].(float64)), nil)
	steward.must(204, code, "author removes")
}

// TestPairingCodeWithSpace: the code is shown as "883 559" and pasted with it.
func TestPairingCodeWithSpace(t *testing.T) {
	srv, _, _, volk := newVillage(t)
	_, obj, _ := volk.do("POST", "/api/pair", nil)
	pin := obj["code"].(string)
	phone := &client{t: t, h: srv.Handler()}
	code, _, _ := phone.do("POST", "/api/join", map[string]any{"code": " " + pin[:3] + " " + pin[3:] + " "})
	phone.must(201, code, "join with a spaced code")
}

// TestMovedTimeIsLoud: a moved date drops the answer out of the headcount,
// tells the houses that answered, and a note-only update keeps their answer.
func TestMovedTimeIsLoud(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push.example/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	_, obj, _ := volk.do("POST", "/api/events", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-20T08:00"})
	id := itoa(obj["id"].(float64))
	// The creation push runs in a goroutine; let it land before the sender is
	// cleared, or on a slow runner it arrives after and counts as the move's.
	waitFor(t, 1, fake)
	steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "yes"})
	_, _, evs := volk.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 2 { // the caller, and the house that said yes
		t.Fatalf("fresh yes not counted: %v", evs[0])
	}

	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = volk.do("PUT", "/api/events/"+id, map[string]any{"starts_at": "2026-09-27T08:00"})
	volk.must(204, code, "move")
	waitFor(t, 1, fake) // the house that said yes hears
	fake.mu.Lock()
	var p Payload
	json.Unmarshal(fake.payloads[0], &p)
	fake.mu.Unlock()
	if !strings.Contains(p.Title, "Termin premaknjen") {
		t.Fatalf("move push: %+v", p)
	}
	_, _, evs = volk.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 0 {
		t.Fatalf("stale answer still counted: %v", evs[0])
	}

	// A body that carries no answer is refused — it is never read as a yes.
	steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "no"})
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"note": "morda pozneje"})
	steward.must(400, code, "no answer in the body")
	_, _, evs = volk.do("GET", "/api/events", nil)
	var list []map[string]any
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if list[0]["state"] != "no" || evs[0]["signups"].(float64) != 0 {
		t.Fatalf("refused body changed the answer: %v", list[0])
	}
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "perhaps"})
	steward.must(400, code, "unknown answer")

	// An unknown /api path is a 404, not the page.
	code, _, _ = steward.do("GET", "/api/nope", nil)
	steward.must(404, code, "unknown api path")
}

// TestCodeCommand: the way back in when nobody is logged in — it names a house,
// rotates its invite, and refuses land and ambiguity.
func TestCodeCommand(t *testing.T) {
	srv, _, steward, _ := newVillage(t)
	steward.do("POST", "/api/houses", map[string]any{"name": "Event grounds", "kind": "common"})
	if err := srv.Code("event grounds"); err == nil {
		t.Fatal("a common place was offered an invite")
	}
	if err := srv.Code("nothing like this"); err == nil {
		t.Fatal("unknown house accepted")
	}
	before, _ := srv.st.One(context.Background(), `SELECT code FROM invites WHERE house_id=(SELECT id FROM houses WHERE name='Zeleni Volk')`)
	if err := srv.Code("zeleni volk"); err != nil {
		t.Fatalf("code: %v", err)
	}
	after, _ := srv.st.One(context.Background(), `SELECT code FROM invites WHERE house_id=(SELECT id FROM houses WHERE name='Zeleni Volk')`)
	if after == nil || (before != nil && before["code"] == after["code"]) {
		t.Fatal("invite was not rotated")
	}
	// The printed code is usable, and the old one is not.
	phone := &client{t: t, h: srv.Handler()}
	if code, _, _ := phone.do("POST", "/api/join", map[string]any{"code": after["code"]}); code != 201 {
		t.Fatalf("fresh code unusable: %d", code)
	}
	if before != nil {
		stale := &client{t: t, h: srv.Handler()}
		if code, _, _ := stale.do("POST", "/api/join", map[string]any{"code": before["code"]}); code == 201 {
			t.Fatal("the old link still works")
		}
	}
}

// TestNextEventIsToday pins the shape of the clock in SQL. An event time is
// local wall clock and datetime('now') is UTC with a space, so the old
// comparison read 'T' against ' ' and called every event today upcoming. The
// project's next event is the one still running, not the one that ended.
func TestNextEventIsToday(t *testing.T) {
	_, _, _, volk := newVillage(t)
	at := func(d time.Duration) string { return time.Now().Add(d).Format("2006-01-02T15:04") }

	code, obj, _ := volk.do("POST", "/api/projects", map[string]any{"title": "Beton"})
	volk.must(201, code, "project")
	pid := obj["id"].(float64)

	code, _, _ = volk.do("POST", "/api/events", map[string]any{
		"title": "over", "kind": "work", "starts_at": at(-4 * time.Hour), "ends_at": at(-2 * time.Hour), "project_id": pid})
	volk.must(201, code, "finished event")
	code, _, _ = volk.do("POST", "/api/events", map[string]any{
		"title": "running", "kind": "work", "starts_at": at(-time.Hour), "ends_at": at(2 * time.Hour), "project_id": pid})
	volk.must(201, code, "running event")

	_, _, list := volk.do("GET", "/api/projects", nil)
	if len(list) != 1 {
		t.Fatalf("want 1 project got %d", len(list))
	}
	if got := list[0]["next_event"]; got != at(-time.Hour) {
		t.Fatalf("next_event = %v, want the running event at %s", got, at(-time.Hour))
	}
	// The list's tally counts every event, the finished one too, and says a
	// project with no pictures has none rather than leaving the field out.
	if list[0]["events"] != 2.0 || list[0]["photos"] != 0.0 {
		t.Fatalf("tally: events=%v photos=%v", list[0]["events"], list[0]["photos"])
	}
}

// TestExpiredPairingIsSwept: expires_at is RFC3339, so the sweep must compare
// in that shape. A dead code of another house goes when anyone makes a new one.
// The dead code died a minute ago, not years ago: a code lives 15 minutes, so
// the sweep almost always compares two stamps of the same day, where the shape
// mismatch ('T' against ' ') is the whole comparison.
func TestExpiredPairingIsSwept(t *testing.T) {
	srv, _, steward, volk := newVillage(t)
	_, me, _ := volk.do("GET", "/api/me", nil)
	dead := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if _, err := srv.st.Exec(context.Background(),
		`INSERT INTO pairings(code, house_id, expires_at) VALUES ('000000', ?, ?)`, int64(me["id"].(float64)), dead); err != nil {
		t.Fatal(err)
	}
	code, _, _ := steward.do("POST", "/api/pair", nil)
	steward.must(201, code, "pair")
	row, err := srv.st.One(context.Background(), `SELECT count(*) AS n FROM pairings WHERE code='000000'`)
	if err != nil {
		t.Fatal(err)
	}
	if row["n"].(int64) != 0 {
		t.Fatalf("expired pairing survived: %v", row)
	}
}

// TestHouseColourIsHex: a colour is painted into an inline style on every
// crest, and CSS `background` would also take `url(...)` — a beacon to a host
// of the house's choosing, fired from every villager's browser. Only #rrggbb.
func TestHouseColourIsHex(t *testing.T) {
	_, _, steward, other := newVillage(t)
	_, me, _ := other.do("GET", "/api/me", nil)
	id := int64(me["id"].(float64))
	code, _, _ := other.do("PUT", fmtID("/api/houses/%d", id), map[string]any{"color": "url(https://x.example/p)"})
	other.must(400, code, "url colour")
	code, _, _ = other.do("PUT", fmtID("/api/houses/%d", id), map[string]any{"color": "#ABCDEF"})
	other.must(204, code, "hex colour")
	code, _, _ = steward.do("POST", "/api/houses", map[string]any{"name": "X", "color": "red"})
	steward.must(400, code, "named colour on create")
}

// TestPushEndpointIsPublicHTTPS: the backend POSTs to whatever a phone
// registers, so it must look like a push service — https, a named host with a
// dot — and never a neighbour on the compose network or a bare address.
func TestPushEndpointIsPublicHTTPS(t *testing.T) {
	_, _, _, other := newVillage(t)
	keys := map[string]any{"p256dh": "k", "auth": "a"}
	for _, bad := range []string{"http://fcm.googleapis.com/x", "https://api:8787/track", "https://caddy/", "https://10.0.0.5/", "https://[::1]/", "https://u@fcm.googleapis.com/x", "file:///etc/passwd"} {
		code, _, _ := other.do("POST", "/api/push/subscribe", map[string]any{"endpoint": bad, "keys": keys})
		other.must(400, code, bad)
	}
	code, _, _ := other.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://fcm.googleapis.com/fcm/send/abc", "keys": keys})
	other.must(204, code, "real endpoint")
}

// TestCommonPlaceGetsNoInvite: land is not an account, on the invite routes
// too — createHouse already skips the invite, and a steward must not be able
// to mint one afterwards.
func TestCommonPlaceGetsNoInvite(t *testing.T) {
	_, _, steward, _ := newVillage(t)
	_, o, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Parkirišče", "kind": "common"})
	id := int64(o["id"].(float64))
	code, _, _ := steward.do("POST", fmtID("/api/houses/%d/invite", id), nil)
	steward.must(400, code, "rotate invite for land")
	code, _, _ = steward.do("GET", fmtID("/api/houses/%d/invite", id), nil)
	steward.must(400, code, "read invite for land")
	code, _, _ = steward.do("GET", "/api/houses/9999/invite", nil)
	steward.must(404, code, "invite for nobody")
}

// TestPageCarriesCSP: the document says it talks to nobody but itself.
func TestPageCarriesCSP(t *testing.T) {
	srv, _, _, _ := newVillage(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") || strings.Contains(got, "http") {
		t.Fatalf("csp: %q", got)
	}
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/status", nil))
	if rec.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("csp on an api answer")
	}
}

func fmtID(f string, id int64) string { return fmt.Sprintf(f, id) }

// TestProjectPhotos: any house puts a picture on a project; the project lists
// it without the bytes; the bytes need a token; the uploader, the project's
// house or a steward removes it; deleting the project takes its pictures with
// it; the export never carries the blob.
func TestProjectPhotos(t *testing.T) {
	srv, _, steward, volk := newVillage(t)
	_, pr, _ := steward.do("POST", "/api/projects", map[string]any{"title": "Ograja"})
	pid := itoa(pr["id"].(float64))
	upload := func(c *client, body string) (int, map[string]any) {
		req := httptest.NewRequest("POST", "/api/projects/"+pid+"/photos", bytes.NewReader([]byte(body)))
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "image/jpeg")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		var o map[string]any
		json.Unmarshal(rec.Body.Bytes(), &o)
		return rec.Code, o
	}
	code, a := upload(volk, "\xff\xd8one")
	if code != 201 {
		t.Fatalf("upload by another house: %d", code)
	}
	code, b := upload(steward, "\xff\xd8two")
	if code != 201 {
		t.Fatalf("upload by owner: %d", code)
	}
	_, p, _ := volk.do("GET", "/api/projects/"+pid, nil)
	photos := p["photos"].([]any)
	if len(photos) != 2 || photos[0].(map[string]any)["house_name"] != "Zeleni Volk" {
		t.Fatalf("photos: %v", photos)
	}
	if _, ok := photos[0].(map[string]any)["photo"]; ok {
		t.Fatal("project leaks the blob")
	}
	// The list counts the pictures without carrying them.
	_, _, list := volk.do("GET", "/api/projects", nil)
	if list[0]["photos"] != 2.0 {
		t.Fatalf("photo tally: %v", list[0]["photos"])
	}
	req := httptest.NewRequest("GET", "/api/photos/"+itoa(a["id"].(float64)), nil)
	req.Header.Set("Authorization", "Bearer "+steward.token)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" || rec.Body.String() != "\xff\xd8one" {
		t.Fatalf("photo get: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/photos/"+itoa(a["id"].(float64)), nil))
	if rec.Code != 401 {
		t.Fatalf("photo without token: %d", rec.Code)
	}
	code, _, _ = volk.do("DELETE", "/api/photos/"+itoa(b["id"].(float64)), nil)
	volk.must(403, code, "delete the owner's picture")
	code, _, _ = steward.do("DELETE", "/api/photos/"+itoa(a["id"].(float64)), nil)
	steward.must(204, code, "project house deletes a neighbour's picture")
	code, exp, _ := steward.do("GET", "/api/export", nil)
	steward.must(200, code, "export")
	if row := exp["project_photos"].([]any)[0].(map[string]any); row["photo"] != nil || row["photo_type"] != "image/jpeg" {
		t.Fatalf("export: %v", row)
	}
	// A picture outlives the house that added it: Zeleni Volk adds one and leaves.
	code, c := upload(volk, "\xff\xd8three")
	if code != 201 {
		t.Fatalf("upload: %d", code)
	}
	_, me, _ := volk.do("GET", "/api/me", nil)
	code, _, _ = steward.do("DELETE", fmtID("/api/houses/%d", int64(me["id"].(float64))), nil)
	steward.must(204, code, "delete house")
	_, p, _ = steward.do("GET", "/api/projects/"+pid, nil)
	var kept map[string]any
	for _, f := range p["photos"].([]any) {
		if f.(map[string]any)["id"] == c["id"] {
			kept = f.(map[string]any)
		}
	}
	if kept == nil || kept["house_id"] != nil || kept["house_name"] != nil {
		t.Fatalf("picture after the house left: %v", kept)
	}
	code, _, _ = steward.do("GET", "/api/photos/"+itoa(c["id"].(float64)), nil)
	steward.must(200, code, "orphan picture still served")
	code, _, _ = steward.do("DELETE", "/api/projects/"+pid, nil)
	steward.must(204, code, "delete project")
	code, _, _ = steward.do("GET", "/api/photos/"+itoa(b["id"].(float64)), nil)
	steward.must(404, code, "picture gone with the project")
}

// TestMarketEdits: a run's destination, time and notes, and an offer's kind,
// are the poster's to change — or a steward's — and nobody else's. A house
// whose need rides on the run hears when its place or time moves; a notes
// edit rings nobody.
func TestMarketEdits(t *testing.T) {
	_, fake, steward, volk := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push.example/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	_, run, _ := volk.do("POST", "/api/runs", map[string]any{"destination": "Kočevje", "cutoff_at": "2026-09-10T09:00"})
	rid := itoa(run["id"].(float64))
	waitFor(t, 1, fake) // the run itself
	code, _, _ = steward.do("POST", "/api/needs", map[string]any{"text": "kvas", "run_id": run["id"]})
	steward.must(201, code, "need on the run")
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = volk.do("PUT", "/api/runs/"+rid, map[string]any{"notes": "only notes"})
	volk.must(204, code, "notes edit")
	waitFor(t, 0, fake)
	code, _, _ = volk.do("PUT", "/api/runs/"+rid, map[string]any{"destination": "Ribnica", "notes": "Merkur too"})
	volk.must(204, code, "edit own run")
	waitFor(t, 1, fake)
	var pl Payload
	json.Unmarshal(fake.payloads[0], &pl)
	if pl.Title != "🚗 Zeleni Volk spremeni vožnjo v Ribnica" || pl.Body != "odhod v četrtek ob 9:00 — tvoja potreba je na tej vožnji" || pl.URL != "#/market" {
		t.Fatalf("rider push: %q / %q / %q", pl.Title, pl.Body, pl.URL)
	}
	// A steward moves the time: the banner still names the driver, and the
	// steward — who is also a rider — hears nothing about its own edit.
	_, o, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Tretja"})
	third := &client{t: t, h: steward.h}
	_, j, _ := third.do("POST", "/api/join", map[string]any{"code": o["invite"].(map[string]any)["code"]})
	third.token = j["token"].(string)
	code, _, _ = third.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push.example/3", "lang": "en", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	third.must(204, code, "subscribe third")
	code, _, _ = third.do("POST", "/api/needs", map[string]any{"text": "nails", "run_id": run["id"]})
	third.must(201, code, "third's need")
	waitFor(t, 2, fake) // the need rang steward and Zeleni Volk
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("PUT", "/api/runs/"+rid, map[string]any{"cutoff_at": "2026-09-10T14:00"})
	steward.must(204, code, "steward moves the time")
	waitFor(t, 1, fake)
	json.Unmarshal(fake.payloads[0], &pl)
	if fake.sent[0].Endpoint != "https://push.example/3" || pl.Title != "🚗 Zeleni Volk changes the run to Ribnica" || pl.Body != "leaves Thursday at 14:00 — your need rides on it" {
		t.Fatalf("steward edit: %s %q / %q", fake.sent[0].Endpoint, pl.Title, pl.Body)
	}
	code, _, _ = volk.do("PUT", "/api/runs/"+rid, map[string]any{"destination": ""})
	volk.must(400, code, "blank destination")
	_, _, runs := steward.do("GET", "/api/runs", nil)
	if runs[0]["destination"] != "Ribnica" || runs[0]["notes"] != "Merkur too" {
		t.Fatalf("run: %v", runs[0])
	}
	code, _, _ = steward.do("PUT", "/api/runs/"+rid, map[string]any{"notes": "steward may"})
	steward.must(204, code, "steward edits")

	// Calling the run off rings the same riders by the same rule, each in the
	// language its phone subscribed in. The needs outlive the run: run_id is
	// ON DELETE SET NULL, so a rider is told to look for another car, not that
	// its need is gone.
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = volk.do("DELETE", "/api/runs/"+rid, nil)
	volk.must(204, code, "driver calls off the run")
	waitFor(t, 2, fake)
	said := map[string][2]string{}
	fake.mu.Lock()
	for i, sub := range fake.sent {
		var q Payload
		json.Unmarshal(fake.payloads[i], &q)
		said[sub.Endpoint] = [2]string{q.Title, q.Body}
	}
	fake.mu.Unlock()
	if got := said["https://push.example/s"]; got[0] != "🚗 Odpade vožnja hiše Zeleni Volk v Ribnica" || got[1] != "tvoja potreba ostane na tržnici, brez vožnje" {
		t.Fatalf("rider in sl: %q / %q", got[0], got[1])
	}
	if got := said["https://push.example/3"]; got[0] != "🚗 Called off: the run to Ribnica by Zeleni Volk" || got[1] != "your need stays on the market, without a run" {
		t.Fatalf("rider in en: %q / %q", got[0], got[1])
	}
	_, _, left := steward.do("GET", "/api/needs", nil)
	if len(left) != 2 {
		t.Fatalf("needs after the run went: %v", left)
	}
	for _, n := range left {
		if n["run_id"] != nil {
			t.Fatalf("need still points at the run: %v", n)
		}
	}
	code, _, _ = steward.do("DELETE", "/api/runs/"+rid, nil)
	steward.must(404, code, "the run is gone")

	// A steward calls off somebody else's run: the driver and the rider are two
	// houses now, so both exclusions are exercised. The driver hears nothing —
	// the same rule as the edit, and the same open question with it.
	_, run2, _ := volk.do("POST", "/api/runs", map[string]any{"destination": "Trgovina", "cutoff_at": "2026-09-11T08:00"})
	code, _, _ = third.do("POST", "/api/needs", map[string]any{"text": "sol", "run_id": run2["id"]})
	third.must(201, code, "third rides again")
	waitFor(t, 5, fake) // the two cancels above, the new run to both phones, the need to one
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("DELETE", fmtID("/api/runs/%d", int64(run2["id"].(float64))), nil)
	steward.must(204, code, "steward calls off another house's run")
	waitFor(t, 1, fake)
	json.Unmarshal(fake.payloads[0], &pl)
	if fake.sent[0].Endpoint != "https://push.example/3" || pl.Title != "🚗 Called off: the run to Trgovina by Zeleni Volk" {
		t.Fatalf("steward cancel: %s %q", fake.sent[0].Endpoint, pl.Title)
	}

	_, off, _ := steward.do("POST", "/api/offers", map[string]any{"text": "Sadike", "tag": "seeds"})
	oid := itoa(off["id"].(float64))
	code, _, _ = volk.do("PUT", "/api/offers/"+oid, map[string]any{"tag": "surplus"})
	volk.must(403, code, "another house edits the kind")
	code, _, _ = steward.do("PUT", "/api/offers/"+oid, map[string]any{"text": "Sadike paradižnika", "tag": "spaceships"})
	steward.must(204, code, "owner edits")
	_, _, offers := volk.do("GET", "/api/offers", nil)
	if offers[0]["text"] != "Sadike paradižnika" || offers[0]["tag"] != "giveaway" {
		t.Fatalf("offer: %v", offers[0])
	}
}

// TestCodex: any house writes the codex and the section says which one; a
// form that opened on an older stamp is refused, not merged; only a steward
// removes a section; the adopted text is imported once, stamped with the
// council's date and no house; the export carries it.
func TestCodex(t *testing.T) {
	srv, _, steward, other := newVillage(t)
	code, _, _ := other.do("POST", "/api/codex", map[string]any{"body_sl": "brez naslova"})
	other.must(400, code, "a section without a title")
	code, obj, _ := other.do("POST", "/api/codex", map[string]any{"title_sl": "Prvi del", "title_en": "First part", "body_sl": "- Prva misel: …"})
	other.must(201, code, "villager adds a section")
	id := obj["id"].(float64)
	_, _, list := steward.do("GET", "/api/codex", nil)
	if len(list) != 1 || list[0]["house_name"] != "Zeleni Volk" || list[0]["ord"].(float64) != 1 {
		t.Fatalf("section not listed with its house: %v", list)
	}
	opened := list[0]["rev"].(float64)
	code, _, _ = steward.do("PUT", fmt.Sprintf("/api/codex/%d", int(id)), map[string]any{"body_en": "- First thought: …", "rev": opened})
	steward.must(204, code, "steward edits a villager's section")
	_, _, list = other.do("GET", "/api/codex", nil)
	if list[0]["body_en"] != "- First thought: …" || list[0]["body_sl"] != "- Prva misel: …" || list[0]["house_name"] != "S" || list[0]["rev"].(float64) != opened+1 {
		t.Fatalf("edit did not land or the stamp did not move: %v", list[0])
	}
	// A second form, opened before the steward saved, is refused — and its
	// text did not land.
	code, _, _ = other.do("PUT", fmt.Sprintf("/api/codex/%d", int(id)), map[string]any{"body_sl": "- Druga misel: …", "rev": opened})
	other.must(409, code, "a form opened on an older rev is refused")
	_, _, list = other.do("GET", "/api/codex", nil)
	if list[0]["body_sl"] != "- Prva misel: …" {
		t.Fatalf("a refused edit landed anyway: %v", list[0])
	}
	code, _, _ = other.do("PUT", fmt.Sprintf("/api/codex/%d", int(id)), map[string]any{"body_sl": "- Druga misel: …"})
	other.must(204, code, "an edit without a rev is taken as it is")
	code, _, _ = other.do("PUT", "/api/codex/999", map[string]any{"body_sl": "x"})
	other.must(404, code, "no such section")
	code, _, _ = other.do("DELETE", fmt.Sprintf("/api/codex/%d", int(id)), nil)
	other.must(403, code, "a villager may not remove a section")
	code, _, _ = steward.do("DELETE", fmt.Sprintf("/api/codex/%d", int(id)), nil)
	steward.must(204, code, "steward removes a section")

	seed := `{"adopted":"2025-01-26","sections":[{"title_sl":"Uvod","title_en":"Intro","body_sl":"a"},{"title_sl":"Drugi del","body_sl":"b"}]}`
	if err := srv.ImportCodex(strings.NewReader(seed)); err != nil {
		t.Fatalf("import: %v", err)
	}
	if err := srv.ImportCodex(strings.NewReader(seed)); err == nil {
		t.Fatal("a second import went through")
	}
	if err := srv.ImportCodex(strings.NewReader(`{"adopted":"26.01.2025","sections":[{"title_sl":"x"}]}`)); err == nil {
		t.Fatal("a non-ISO date went through")
	}
	_, _, list = other.do("GET", "/api/codex", nil)
	if len(list) != 2 || list[0]["title_sl"] != "Uvod" || list[1]["ord"].(float64) != 2 {
		t.Fatalf("imported sections wrong: %v", list)
	}
	if list[0]["updated_by"] != nil || list[0]["house_name"] != nil || !strings.HasPrefix(list[0]["updated_at"].(string), "2025-01-26") {
		t.Fatalf("imported section should carry the council's date and no house: %v", list[0])
	}
	_, exp, _ := steward.do("GET", "/api/export", nil)
	if rows, ok := exp["codex_sections"].([]any); !ok || len(rows) != 2 {
		t.Fatalf("export lacks the codex: %v", exp["codex_sections"])
	}
}
