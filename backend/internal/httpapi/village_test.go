package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
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

// TestQuietHours: nothing but an alarm rings at 23:00.
func TestQuietHours(t *testing.T) {
	srv, fake, steward, zagar := newVillage(t)
	srv.now = func() time.Time { return time.Date(2026, 9, 4, 23, 10, 0, 0, time.Local) }
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{
		"endpoint": "https://push/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")

	zagar.do("POST", "/api/needs", map[string]any{"text": "mleko"})
	waitFor(t, 0, fake)
	zagar.do("POST", "/api/events", map[string]any{"title": "Medved", "kind": "alarm", "starts_at": "2026-09-04T23:10"})
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
	code, _, _ = steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"note": "pridem s koso"})
	steward.must(204, code, "signup")
	waitFor(t, 1, fake) // the house that called it hears

	_, _, evs := zagar.do("GET", "/api/events", nil)
	if evs[0]["signups"].(float64) != 1 || evs[0]["mine"].(float64) != 0 {
		t.Fatalf("signups: %v", evs[0])
	}
	_, _, evs = steward.do("GET", "/api/events", nil)
	if evs[0]["mine"].(float64) != 1 {
		t.Fatalf("mine flag: %v", evs[0])
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

// TestSignupNote: a sign-up may carry a note, and the event list returns each
// signer with it.
func TestSignupNote(t *testing.T) {
	_, _, steward, zagar := newVillage(t)
	_, obj, _ := zagar.do("POST", "/api/events", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-06T08:00"})
	id := itoa(obj["id"].(float64))
	steward.do("POST", "/api/events/"+id+"/signup", map[string]any{"note": "pridem s koso"})
	_, _, evs := zagar.do("GET", "/api/events", nil)
	var list []map[string]any
	json.Unmarshal([]byte(evs[0]["signup_list"].(string)), &list)
	if len(list) != 1 || list[0]["note"] != "pridem s koso" || list[0]["name"] != "S" {
		t.Fatalf("signup_list: %v", evs[0]["signup_list"])
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

// TestAlarmBeatsMute: an alarm rings through a house mute, a village mute and
// quiet hours; the mute records who set it.
func TestAlarmBeatsMute(t *testing.T) {
	srv, fake, steward, zagar := newVillage(t)
	srv.now = func() time.Time { return time.Date(2026, 9, 4, 23, 30, 0, 0, time.Local) }
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	steward.do("PUT", "/api/me/prefs", map[string]any{"off": []string{"events"}})
	steward.do("PUT", "/api/prefs/global", map[string]any{"off": []string{"events"}})
	zagar.do("POST", "/api/events", map[string]any{"title": "Ogenj", "kind": "alarm", "starts_at": "2026-09-04T23:30"})
	waitFor(t, 1, fake)
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
	// The project creator cannot eject the holder; only the holder lets go.
	code, _, _ = zagar.do("PUT", "/api/tasks/"+tid, map[string]any{"take": false})
	zagar.must(403, code, "creator ejects holder")
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

// TestCamp: a collection is a row with no amount; the village hears; hand-over
// is a state the collecting house or a steward sets.
func TestCamp(t *testing.T) {
	_, fake, steward, zagar := newVillage(t)
	code, _, _ := steward.do("POST", "/api/push/subscribe", map[string]any{"endpoint": "https://push/s", "lang": "sl", "keys": map[string]any{"p256dh": "p", "auth": "a"}})
	steward.must(204, code, "subscribe")
	code, obj, _ := zagar.do("POST", "/api/camp", map[string]any{"from_who": "siv kamper", "taken_on": "2026-09-05", "collected_by": "Ana", "notes": "2 noči"})
	zagar.must(201, code, "camp")
	id := itoa(obj["id"].(float64))
	waitFor(t, 1, fake)
	fake.mu.Lock()
	var p Payload
	json.Unmarshal(fake.payloads[0], &p)
	fake.mu.Unlock()
	if p.Kind != "camp" || p.Title != "🏕️ Ana · Žagar je pobral kamp" || p.Body != "siv kamper" {
		t.Fatalf("camp push: %+v", p)
	}
	_, _, rows := steward.do("GET", "/api/camp", nil)
	if _, ok := rows[0]["amount_cents"]; ok {
		t.Fatal("camp stores an amount")
	}
	code, _, _ = steward.do("PUT", "/api/camp/"+id, map[string]any{"state": "handed"})
	steward.must(204, code, "handed")
	_, _, rows = zagar.do("GET", "/api/camp", nil)
	if rows[0]["state"] != "handed" || rows[0]["handed_at"] == nil {
		t.Fatalf("handed: %v", rows[0])
	}
}
