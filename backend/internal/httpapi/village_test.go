package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/config"
	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/store"
)

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
	_, o, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Žagar"})
	other := &client{t: t, h: h}
	_, j, _ := other.do("POST", "/api/join", map[string]any{"code": o["invite"].(map[string]any)["code"]})
	other.token = j["token"].(string)
	return srv, fake, steward, other
}

// TestPairingCode: a logged-in house makes a six-digit code, a second phone of
// the same house uses it once, and the code then dies.
func TestPairingCode(t *testing.T) {
	srv, _, _, zagar := newVillage(t)
	code, obj, _ := zagar.do("POST", "/api/pair", nil)
	zagar.must(201, code, "pair")
	pin := obj["code"].(string)
	if len(pin) != 6 {
		t.Fatalf("want 6 digits, got %q", pin)
	}
	phone2 := &client{t: t, h: srv.Handler()}
	code, j, _ := phone2.do("POST", "/api/join", map[string]any{"code": pin, "device": "phone B"})
	phone2.must(201, code, "join by pairing")
	phone2.token = j["token"].(string)
	_, me, _ := phone2.do("GET", "/api/me", nil)
	if me["name"] != "Žagar" {
		t.Fatalf("paired into wrong house: %v", me)
	}
	// Single use.
	phone3 := &client{t: t, h: srv.Handler()}
	code, _, _ = phone3.do("POST", "/api/join", map[string]any{"code": pin})
	phone3.must(404, code, "reuse refused")
	// Both phones belong to the house.
	_, _, devs := zagar.do("GET", "/api/devices", nil)
	if len(devs) != 2 {
		t.Fatalf("want 2 devices got %d", len(devs))
	}
}

// TestGuessingCostsSomething: wrong codes are throttled per IP and five misses
// drop every live pairing code.
func TestGuessingCostsSomething(t *testing.T) {
	srv, _, _, zagar := newVillage(t)
	_, obj, _ := zagar.do("POST", "/api/pair", nil)
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
	srv, fake, steward, zagar := newVillage(t)
	srv.now = func() time.Time { return time.Date(2026, 9, 4, 23, 10, 0, 0, time.Local) }
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")

	zagar.do("POST", "/api/needs", map[string]any{"text": "mleko"})
	zagar.do("POST", "/api/events", map[string]any{"title": "Nekaj", "kind": "work", "starts_at": "2026-09-05T09:00"})
	waitFor(t, 0, fake)

	// Daylight: the same event rings.
	srv.now = func() time.Time { return time.Date(2026, 9, 4, 10, 0, 0, 0, time.Local) }
	zagar.do("POST", "/api/events", map[string]any{"title": "Drugo", "kind": "work", "starts_at": "2026-09-05T09:00"})
	waitFor(t, 1, fake)
}

// TestToolShed: a house shares a tool, another takes it, the owner is told,
// and returning frees it.
func TestToolShed(t *testing.T) {
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := zagar.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	zagar.must(204, code, "subscribe")

	code, obj, _ := zagar.do("POST", "/api/tools", map[string]any{"name": "Motorna žaga", "notes": "gorivo svoje"})
	zagar.must(201, code, "create tool")
	id := itoa(obj["id"].(float64))
	waitFor(t, 0, fake) // the sharing house does not hear its own tool

	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("PUT", "/api/tools/"+id, map[string]any{"take": true})
	steward.must(204, code, "take")
	waitFor(t, 1, fake) // only the owner is told
	code, _, _ = zagar.do("PUT", "/api/tools/"+id, map[string]any{"take": true})
	zagar.must(409, code, "double take")

	_, _, tools := zagar.do("GET", "/api/tools", nil)
	if tools[0]["held_by_name"] != "S" {
		t.Fatalf("holder: %v", tools[0])
	}
	code, _, _ = steward.do("PUT", "/api/tools/"+id, map[string]any{"take": false})
	steward.must(204, code, "return")
	_, _, tools = zagar.do("GET", "/api/tools", nil)
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
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := zagar.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	zagar.must(204, code, "subscribe")
	code, obj, _ := zagar.do("POST", "/api/events", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-06T08:00"})
	zagar.must(201, code, "event")
	id := itoa(obj["id"].(float64))

	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "yes"})
	steward.must(204, code, "signup")
	waitFor(t, 1, fake) // the house that called it hears

	_, _, evs := zagar.do("GET", "/api/events", nil)
	// `mine` is this house's own answer, or null when it has not answered.
	if evs[0]["signups"].(float64) != 1 || evs[0]["mine"] != nil {
		t.Fatalf("signups: %v", evs[0])
	}
	_, _, evs = steward.do("GET", "/api/events", nil)
	if evs[0]["mine"] != "yes" {
		t.Fatalf("mine answer: %v", evs[0])
	}
	code, _, _ = steward.do("DELETE", "/api/events/"+id+"/signup", nil)
	steward.must(204, code, "sign off")
	_, _, evs = zagar.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 0 {
		t.Fatalf("still signed up: %v", evs[0])
	}
}

// TestToolReminder: a tool out for two days nags its holder once a day, the
// owner hears nothing, and returning it clears the clock.
func TestToolReminder(t *testing.T) {
	srv, fake, steward, zagar := newVillage(t)
	for _, c := range []struct {
		c  *client
		ep string
	}{{steward, "https://push/s"}, {zagar, "https://push/z"}} {
		code, _, _ := c.c.do("POST", "/api/push/subscribe", map[string]any{"endpoint": c.ep, "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
		c.c.must(204, code, "subscribe")
	}
	_, obj, _ := zagar.do("POST", "/api/tools", map[string]any{"name": "Lestev"})
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
	if fake.sent[0].Endpoint != "https://push/s" {
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
	srv, _, steward, zagar := newVillage(t)
	code, obj, _ := zagar.do("POST", "/api/tools", map[string]any{"name": "Kosilnica", "category": "garden"})
	zagar.must(201, code, "tool")
	id := itoa(obj["id"].(float64))

	// Photo: raw bytes with a type; the steward may not upload to Žagar's tool? Stewards may.
	req := httptest.NewRequest("PUT", "/api/tools/"+id+"/photo", bytes.NewReader([]byte("\xff\xd8jpegbytes")))
	req.Header.Set("Authorization", "Bearer "+zagar.token)
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
	code, _, _ = zagar.do("PUT", "/api/tools/"+id, map[string]any{"category": "spaceships"})
	zagar.must(204, code, "category")
	_, _, tools = zagar.do("GET", "/api/tools", nil)
	if tools[0]["category"] != "other" {
		t.Fatalf("category: %v", tools[0]["category"])
	}

	// Wishlist.
	code, obj, _ = zagar.do("POST", "/api/wishes", map[string]any{"text": "Cepilec drv"})
	zagar.must(201, code, "wish")
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
	_, _, steward, zagar := newVillage(t)
	_, obj, _ := zagar.do("POST", "/api/events", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-06T08:00"})
	id := itoa(obj["id"].(float64))
	code, _, _ := steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"note": "pridem s koso"})
	steward.must(400, code, "a note is not an answer")
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "yes", "note": "pridem s koso"})
	steward.must(204, code, "signup")
	_, _, evs := zagar.do("GET", "/api/events", nil)
	var list []map[string]any
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if len(list) != 1 || list[0]["name"] != "S" {
		t.Fatalf("signup_list: %v", evs[0]["signup_list"])
	}
	if _, ok := list[0]["note"]; ok {
		t.Fatalf("sign-up still carries a note: %v", list[0])
	}
}

// TestQuietHoursOptOut: a phone that ticked "ring at night" hears the same
// event that the rest of the village sleeps through, and its house-mates keep
// sleeping — the flag is on the device, not on the house.
func TestQuietHoursOptOut(t *testing.T) {
	srv, fake, steward, zagar := newVillage(t)
	// Two phones of the steward's house: the second one asks for night rings.
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push/s1", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe phone 1")
	_, pair, _ := steward.do("POST", "/api/pair", nil)
	phone2 := &client{t: t, h: srv.Handler()}
	_, j, _ := phone2.do("POST", "/api/join", map[string]any{"code": pair["code"].(string), "device": "night phone"})
	phone2.token = j["token"].(string)
	code, _, _ = phone2.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push/s2", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	phone2.must(204, code, "subscribe phone 2")
	code, _, _ = phone2.do("PUT", "/api/me/device", map[string]any{"quiet_ok": true})
	phone2.must(204, code, "opt out of quiet hours")

	srv.now = func() time.Time { return time.Date(2026, 9, 4, 23, 10, 0, 0, time.Local) }
	zagar.do("POST", "/api/events", map[string]any{"title": "Nekaj", "kind": "work", "starts_at": "2026-09-05T09:00"})
	waitFor(t, 1, fake)
	fake.mu.Lock()
	got := []string{}
	for _, sub := range fake.sent {
		got = append(got, sub.Endpoint)
	}
	fake.mu.Unlock()
	if len(got) != 1 || got[0] != "https://push/s2" {
		t.Fatalf("night push went to %v, want only the phone that asked", got)
	}

	// The house's own page reports the flag for THIS phone only.
	_, p2, _ := phone2.do("GET", "/api/me/prefs", nil)
	_, p1, _ := steward.do("GET", "/api/me/prefs", nil)
	if p2["quiet_ok"] != true || p1["quiet_ok"] != false {
		t.Fatalf("quiet_ok leaked across phones: %v %v", p1["quiet_ok"], p2["quiet_ok"])
	}
}

// TestGlobalMute: a steward mutes a kind for the whole village; a villager
// cannot; the house list still shows it as off.
func TestGlobalMute(t *testing.T) {
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	code, _, _ = zagar.do("PUT", "/api/prefs/global", map[string]any{"off": []string{"needs"}})
	zagar.must(403, code, "villager mutes globally")
	code, _, _ = steward.do("PUT", "/api/prefs/global", map[string]any{"off": []string{"needs"}})
	steward.must(204, code, "steward mutes needs")
	_, prefs, _ := zagar.do("GET", "/api/me/prefs", nil)
	if g := prefs["global_off"].([]any); len(g) != 1 || g[0] != "needs" {
		t.Fatalf("global_off: %v", prefs)
	}
	zagar.do("POST", "/api/needs", map[string]any{"text": "sol"})
	waitFor(t, 0, fake)
	zagar.do("POST", "/api/offers", map[string]any{"text": "okna"})
	waitFor(t, 1, fake)
}

// TestMuteReachesNobody: a village-wide mute silences a kind for every house,
// and it records which steward set it and when.
func TestMuteReachesNobody(t *testing.T) {
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	steward.do("PUT", "/api/prefs/global", map[string]any{"off": []string{"events"}})
	zagar.do("POST", "/api/events", map[string]any{"title": "Nekaj", "kind": "work", "starts_at": "2026-09-05T09:00"})
	waitFor(t, 0, fake)
	_, prefs, _ := zagar.do("GET", "/api/me/prefs", nil)
	d := prefs["global_detail"].([]any)[0].(map[string]any)
	if d["kind"] != "events" || d["set_by"] != "S" || d["set_at"] == nil {
		t.Fatalf("global_detail: %v", d)
	}
}

// TestProjects: a project with a takable task, an event linked to it, the
// creator told when a task is taken, done as a state, export complete.
func TestProjects(t *testing.T) {
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := zagar.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	zagar.must(204, code, "subscribe")

	code, obj, _ := zagar.do("POST", "/api/projects", map[string]any{"title": "Ograja okoli travnika", "due_at": "2026-10-15", "notes": "300 m"})
	zagar.must(201, code, "project")
	pid := itoa(obj["id"].(float64))
	waitFor(t, 0, fake) // author does not hear its own project
	code, obj, _ = zagar.do("POST", "/api/projects/"+pid+"/tasks", map[string]any{"title": "Kupiti stebre", "due_at": "2026-09-20"})
	zagar.must(201, code, "task")
	tid := itoa(obj["id"].(float64))

	// Steward takes the task; Žagar (project creator) hears.
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("PUT", "/api/tasks/"+tid, map[string]any{"take": true})
	steward.must(204, code, "take")
	waitFor(t, 1, fake)
	code, _, _ = zagar.do("PUT", "/api/tasks/"+tid, map[string]any{"take": true})
	zagar.must(409, code, "double take")

	// A third house cannot close it; the holder can, with a note.
	_, o, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Tretja"})
	third := &client{t: t, h: zagar.h}
	_, j, _ := third.do("POST", "/api/join", map[string]any{"code": o["invite"].(map[string]any)["code"]})
	third.token = j["token"].(string)
	code, _, _ = third.do("PUT", "/api/tasks/"+tid, map[string]any{"state": "done"})
	third.must(403, code, "stranger closes")
	// A stranger cannot clear the holder; the creator may (things get agreed
	// in real life), and may assign — the assigned house hears.
	_, o2, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Cetrta"})
	fourth := &client{t: t, h: zagar.h}
	_, j2, _ := fourth.do("POST", "/api/join", map[string]any{"code": o2["invite"].(map[string]any)["code"]})
	fourth.token = j2["token"].(string)
	code, _, _ = fourth.do("PUT", "/api/tasks/"+tid, map[string]any{"take": false})
	fourth.must(403, code, "stranger clears holder")
	code, _, _ = fourth.do("PUT", "/api/tasks/"+tid, map[string]any{"assigned_to": 1})
	fourth.must(403, code, "stranger assigns")
	code, _, _ = fourth.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push/4", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	fourth.must(204, code, "subscribe 4th")
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	_, me4, _ := fourth.do("GET", "/api/me", nil)
	code, _, _ = zagar.do("PUT", "/api/tasks/"+tid, map[string]any{"assigned_to": me4["id"]})
	zagar.must(204, code, "creator assigns")
	waitFor(t, 1, fake)
	code, _, _ = zagar.do("PUT", "/api/tasks/"+tid, map[string]any{"take": false})
	zagar.must(204, code, "creator clears")
	code, _, _ = steward.do("PUT", "/api/tasks/"+tid, map[string]any{"take": true})
	steward.must(204, code, "steward takes again")
	code, _, _ = steward.do("PUT", "/api/tasks/"+tid, map[string]any{"state": "done", "closing_note": "kupljeno pri Bauhausu"})
	steward.must(204, code, "holder closes")
	code, _, _ = zagar.do("PUT", "/api/tasks/"+tid, map[string]any{"take": true})
	zagar.must(409, code, "take a done task")

	// Event linked to the task inherits the project.
	code, _, _ = zagar.do("POST", "/api/events", map[string]any{"title": "Postavljanje", "kind": "work", "starts_at": "2026-09-27T09:00", "task_id": json.Number(tid)})
	zagar.must(201, code, "event on task")
	_, _, evs := zagar.do("GET", "/api/events", nil)
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
	code, _, _ = zagar.do("PUT", "/api/projects/"+pid, map[string]any{"state": "done"})
	zagar.must(204, code, "finish project")
	code, _, _ = zagar.do("PUT", "/api/projects/"+pid, map[string]any{"state": "open"})
	zagar.must(204, code, "reopen")
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
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	code, obj, _ := zagar.do("POST", "/api/camp", map[string]any{"notes": "siv kamper"})
	zagar.must(201, code, "arrived")
	id := itoa(obj["id"].(float64))
	waitFor(t, 1, fake)
	fake.mu.Lock()
	var p Payload
	json.Unmarshal(fake.payloads[0], &p)
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	if p.Kind != "camp" || p.Title != "🏕️ Žagar: kamper je prišel" || p.Body != "siv kamper" {
		t.Fatalf("arrival push: %+v", p)
	}
	// Steward cannot hand over what nobody holds; steward claims; Žagar hears... steward is the only phone here.
	code, _, _ = steward.do("PUT", "/api/camp/"+id, map[string]any{"state": "handed"})
	steward.must(409, code, "hand over unclaimed")
	code, _, _ = steward.do("PUT", "/api/camp/"+id, map[string]any{"claim": true, "notes": "2 noči"})
	steward.must(204, code, "claim")
	_, _, rows := zagar.do("GET", "/api/camp", nil)
	if rows[0]["state"] != "held" || rows[0]["held_by_name"] != "S" || rows[0]["notes"] != "siv kamper" {
		t.Fatalf("after claim (note must not overwrite): %v", rows[0])
	}
	code, _, _ = zagar.do("PUT", "/api/camp/"+id, map[string]any{"claim": true})
	zagar.must(409, code, "double claim")
	code, _, _ = zagar.do("PUT", "/api/camp/"+id, map[string]any{"state": "handed"})
	zagar.must(403, code, "non-holder hands over")
	code, _, _ = steward.do("PUT", "/api/camp/"+id, map[string]any{"state": "handed"})
	steward.must(204, code, "holder hands over")
	// Tick "I have the money": straight to handed, holder = noticer.
	code, _, _ = zagar.do("POST", "/api/camp", map[string]any{"notes": "NL družina", "have_money": true})
	zagar.must(201, code, "arrived with money")
	_, _, rows = zagar.do("GET", "/api/camp", nil)
	var withMoney map[string]any
	for _, r := range rows {
		if r["notes"] == "NL družina" {
			withMoney = r
		}
	}
	if withMoney["state"] != "handed" || withMoney["held_by_name"] != "Žagar" || withMoney["handed_at"] == nil {
		t.Fatalf("have_money row: %v", withMoney)
	}
	if _, ok := rows[0]["amount_cents"]; ok {
		t.Fatal("camp stores an amount")
	}
}

// TestRsvpAndComments: three answers not one, a moved date marks answers stale,
// any house edits an event, comments thread one level and ring the caller.
func TestRsvpAndComments(t *testing.T) {
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := zagar.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	zagar.must(204, code, "subscribe")
	code, obj, _ := zagar.do("POST", "/api/events", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-20T08:00"})
	zagar.must(201, code, "event")
	id := itoa(obj["id"].(float64))

	// "not coming" is an answer, not silence: it is stored and it is not a yes.
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "no"})
	steward.must(204, code, "rsvp no")
	_, _, evs := zagar.do("GET", "/api/events", nil)
	var list []map[string]any
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if evs[0]["signups"].(float64) != 0 || list[0]["state"] != "no" {
		t.Fatalf("no-answer counted as coming: %v", evs[0])
	}
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "maybe"})
	steward.must(204, code, "rsvp maybe")
	_, _, evs = zagar.do("GET", "/api/events", nil)
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if evs[0]["signups"].(float64) != 0 || list[0]["state"] != "maybe" || list[0]["stale"].(float64) != 0 {
		t.Fatalf("maybe folded into coming: %v %v", evs[0], list[0])
	}

	// Any house edits; moving the time marks earlier answers stale.
	code, _, _ = steward.do("PUT", "/api/events/"+id, map[string]any{"notes": "prinesite grablje"})
	steward.must(204, code, "any house edits")
	_, _, evs = zagar.do("GET", "/api/events", nil)
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if evs[0]["edited_by_name"] != "S" || list[0]["stale"].(float64) != 0 {
		t.Fatalf("note edit made answers stale: %v", evs[0])
	}
	code, _, _ = steward.do("PUT", "/api/events/"+id, map[string]any{"starts_at": "2026-09-21T08:00"})
	steward.must(204, code, "move the date")
	_, _, evs = zagar.do("GET", "/api/events", nil)
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if list[0]["stale"].(float64) != 1 {
		t.Fatalf("answer survived a moved date as current: %v", list[0])
	}

	// Comments: one reply level, a reply to a reply hangs off the root.
	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, c1, _ := steward.do("POST", "/api/threads/event/"+id, map[string]any{"body": "Kdaj točno?", "author": "Ana"})
	steward.must(201, code, "comment")
	waitFor(t, 1, fake) // the house that called it hears
	rootID := itoa(c1["id"].(float64))
	_, c2, _ := zagar.do("POST", "/api/threads/event/"+id, map[string]any{"body": "Ob osmih", "parent_id": c1["id"]})
	_, c3, _ := steward.do("POST", "/api/threads/event/"+id, map[string]any{"body": "Prav", "parent_id": c2["id"]})
	_, _, cs := zagar.do("GET", "/api/threads/event/"+id, nil)
	if len(cs) != 3 {
		t.Fatalf("want 3 comments got %d", len(cs))
	}
	for _, c := range cs {
		if c["id"].(float64) == c3["id"].(float64) && itoa(c["parent_id"].(float64)) != rootID {
			t.Fatalf("reply to a reply did not flatten: %v", c)
		}
	}
	_, _, evs = zagar.do("GET", "/api/events", nil)
	if evs[0]["comments"].(float64) != 3 {
		t.Fatalf("comment count: %v", evs[0]["comments"])
	}
	// Only the author or a steward deletes a comment.
	code, _, _ = zagar.do("DELETE", "/api/comments/"+rootID, nil)
	zagar.must(403, code, "stranger deletes a comment")
	code, _, _ = steward.do("DELETE", "/api/comments/"+rootID, nil)
	steward.must(204, code, "author deletes")
	_, _, cs = zagar.do("GET", "/api/threads/event/"+id, nil)
	if len(cs) != 0 {
		t.Fatalf("cascade left %d comments", len(cs))
	}
}

// TestWishOptionsAndThread: the same thread on a wish, plus options anyone can
// add. An option is a finding, never a vote — nothing counts or ranks them.
func TestWishOptionsAndThread(t *testing.T) {
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := zagar.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push/z", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	zagar.must(204, code, "subscribe")
	code, obj, _ := zagar.do("POST", "/api/wishes", map[string]any{"text": "Cepilec drv"})
	zagar.must(201, code, "wish")
	wid := itoa(obj["id"].(float64))

	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = steward.do("POST", "/api/wishes/"+wid+"/options", map[string]any{"text": "Vevor 7t, 180 EUR", "url": "example.com/vevor"})
	steward.must(201, code, "option")
	waitFor(t, 1, fake)
	code, _, _ = steward.do("POST", "/api/wishes/"+wid+"/options", map[string]any{"text": ""})
	steward.must(400, code, "empty option")

	_, _, wishes := zagar.do("GET", "/api/wishes", nil)
	var opts []map[string]any
	json.Unmarshal([]byte(wishes[0]["options"].(string)), &opts)
	if len(opts) != 1 || opts[0]["url"] != "https://example.com/vevor" || opts[0]["name"] != "S" {
		t.Fatalf("options: %v", wishes[0]["options"])
	}
	code, c1, _ := steward.do("POST", "/api/threads/wish/"+wid, map[string]any{"body": "Videl sem cenejšega"})
	steward.must(201, code, "wish comment")
	_, c2, _ := zagar.do("POST", "/api/threads/wish/"+wid, map[string]any{"body": "Kje?", "parent_id": c1["id"]})
	_, c3, _ := steward.do("POST", "/api/threads/wish/"+wid, map[string]any{"body": "V Kočevju", "parent_id": c2["id"]})
	_, _, cs := zagar.do("GET", "/api/threads/wish/"+wid, nil)
	if len(cs) != 3 {
		t.Fatalf("want 3 got %d", len(cs))
	}
	for _, c := range cs {
		if c["id"].(float64) == c3["id"].(float64) && c["parent_id"].(float64) != c1["id"].(float64) {
			t.Fatalf("reply to a reply did not flatten: %v", c)
		}
	}
	_, _, wishes = zagar.do("GET", "/api/wishes", nil)
	if wishes[0]["comments"].(float64) != 3 {
		t.Fatalf("comment count: %v", wishes[0]["comments"])
	}
	code, _, _ = steward.do("POST", "/api/threads/event/"+wid, map[string]any{"body": "x"})
	steward.must(404, code, "comment on a missing event")
	code, _, _ = steward.do("POST", "/api/threads/nonsense/1", map[string]any{"body": "x"})
	steward.must(400, code, "unknown subject")
	code, _, _ = zagar.do("DELETE", "/api/options/"+itoa(opts[0]["id"].(float64)), nil)
	zagar.must(403, code, "stranger removes an option")
	code, _, _ = steward.do("DELETE", "/api/options/"+itoa(opts[0]["id"].(float64)), nil)
	steward.must(204, code, "author removes")
}

// TestPairingCodeWithSpace: the code is shown as "883 559" and pasted with it.
func TestPairingCodeWithSpace(t *testing.T) {
	srv, _, _, zagar := newVillage(t)
	_, obj, _ := zagar.do("POST", "/api/pair", nil)
	pin := obj["code"].(string)
	phone := &client{t: t, h: srv.Handler()}
	code, _, _ := phone.do("POST", "/api/join", map[string]any{"code": " " + pin[:3] + " " + pin[3:] + " "})
	phone.must(201, code, "join with a spaced code")
}

// TestMovedTimeIsLoud: a moved date drops the answer out of the headcount,
// tells the houses that answered, and a note-only update keeps their answer.
func TestMovedTimeIsLoud(t *testing.T) {
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	_, obj, _ := zagar.do("POST", "/api/events", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-20T08:00"})
	id := itoa(obj["id"].(float64))
	steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "yes"})
	_, _, evs := zagar.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 1 {
		t.Fatalf("fresh yes not counted: %v", evs[0])
	}

	fake.mu.Lock()
	fake.sent, fake.payloads = nil, nil
	fake.mu.Unlock()
	code, _, _ = zagar.do("PUT", "/api/events/"+id, map[string]any{"starts_at": "2026-09-27T08:00"})
	zagar.must(204, code, "move")
	waitFor(t, 1, fake) // the house that said yes hears
	fake.mu.Lock()
	var p Payload
	json.Unmarshal(fake.payloads[0], &p)
	fake.mu.Unlock()
	if !strings.Contains(p.Title, "Termin premaknjen") {
		t.Fatalf("move push: %+v", p)
	}
	_, _, evs = zagar.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 0 {
		t.Fatalf("stale answer still counted: %v", evs[0])
	}

	// A body that carries no answer is refused — it is never read as a yes.
	steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"state": "no"})
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"note": "morda pozneje"})
	steward.must(400, code, "no answer in the body")
	_, _, evs = zagar.do("GET", "/api/events", nil)
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
	before, _ := srv.st.One(context.Background(), `SELECT code FROM invites WHERE house_id=(SELECT id FROM houses WHERE name='Žagar')`)
	if err := srv.Code("žagar"); err != nil {
		t.Fatalf("code: %v", err)
	}
	after, _ := srv.st.One(context.Background(), `SELECT code FROM invites WHERE house_id=(SELECT id FROM houses WHERE name='Žagar')`)
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
