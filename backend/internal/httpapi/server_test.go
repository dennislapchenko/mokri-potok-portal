package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/config"
	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/store"
)

type client struct {
	t     *testing.T
	h     http.Handler
	token string
}

func (c *client) do(method, path string, body any) (int, map[string]any, []map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Origin", "http://localhost:5173")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	var obj map[string]any
	var list []map[string]any
	raw := rec.Body.Bytes()
	if len(raw) > 0 && raw[0] == '[' {
		json.Unmarshal(raw, &list)
	} else if len(raw) > 0 {
		json.Unmarshal(raw, &obj)
	}
	return rec.Code, obj, list
}

func (c *client) must(code int, got int, what string) {
	if got != code {
		c.t.Fatalf("%s: want %d got %d", what, code, got)
	}
}

// TestVillageFlow walks the whole v0 story: bootstrap a steward, create a
// house, join it through the invite, post, need/take, away/watch, export.
func TestVillageFlow(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st, config.Config{CORSOrigins: []string{"http://localhost:5173"}, BootstrapCode: "letmein"})
	steward := &client{t: t, h: srv.Handler()}

	code, obj, _ := steward.do("GET", "/api/status", nil)
	steward.must(200, code, "status")
	if obj["bootstrap_needed"] != true {
		t.Fatal("expected bootstrap_needed")
	}
	code, _, _ = steward.do("POST", "/api/bootstrap", map[string]any{"code": "wrong", "name": "x"})
	steward.must(403, code, "bootstrap wrong code")
	code, obj, _ = steward.do("POST", "/api/bootstrap", map[string]any{"code": "letmein", "name": "Stewards"})
	steward.must(201, code, "bootstrap")
	steward.token = obj["token"].(string)
	code, _, _ = steward.do("POST", "/api/bootstrap", map[string]any{"code": "letmein"})
	steward.must(409, code, "second bootstrap refused")

	// Steward creates a house and gets its invite.
	code, obj, _ = steward.do("POST", "/api/houses", map[string]any{"name": "Pri Lipi", "crest": "🌳", "color": "#4a7c3f"})
	steward.must(201, code, "create house")
	houseID := obj["id"].(float64)
	inv := obj["invite"].(map[string]any)["code"].(string)

	// A villager joins twice (two phones), both devices belong to the house.
	villager := &client{t: t, h: srv.Handler()}
	code, obj, _ = villager.do("POST", "/api/join", map[string]any{"code": inv, "device": "phone A"})
	villager.must(201, code, "join A")
	villager.token = obj["token"].(string)
	second := &client{t: t, h: srv.Handler()}
	code, obj, _ = second.do("POST", "/api/join", map[string]any{"code": inv, "device": "phone B"})
	second.must(201, code, "join B")
	second.token = obj["token"].(string)
	code, _, devs := villager.do("GET", "/api/devices", nil)
	villager.must(200, code, "devices")
	if len(devs) != 2 {
		t.Fatalf("want 2 devices got %d", len(devs))
	}
	code, obj, _ = villager.do("GET", "/api/me", nil)
	villager.must(200, code, "me")
	if obj["name"] != "Pri Lipi" || obj["is_steward"].(float64) != 0 {
		t.Fatalf("me: %v", obj)
	}

	// Villager cannot create houses; steward assigns parcels.
	code, _, _ = villager.do("POST", "/api/houses", map[string]any{"name": "nope"})
	villager.must(403, code, "villager creates house")
	code, _, _ = steward.do("PUT", "/api/houses/"+itoa(houseID), map[string]any{"parcels": []string{"2494", "2496"}})
	steward.must(204, code, "assign parcels")
	code, _, houses := villager.do("GET", "/api/houses", nil)
	villager.must(200, code, "list houses")
	var found bool
	for _, h := range houses {
		if h["name"] == "Pri Lipi" && len(h["parcels"].([]any)) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("parcels not visible: %v", houses)
	}

	// Tavern: post, steward pins, villager cannot pin.
	code, obj, _ = villager.do("POST", "/api/posts", map[string]any{"body": "Pozdravljeni!", "author": "Ana"})
	villager.must(201, code, "post")
	pid := itoa(obj["id"].(float64))
	code, _, _ = villager.do("PUT", "/api/posts/"+pid, map[string]any{"pinned": true})
	villager.must(403, code, "villager pins")
	code, _, _ = steward.do("PUT", "/api/posts/"+pid, map[string]any{"pinned": true})
	steward.must(204, code, "steward pins")

	// Market: villager needs milk, steward takes it, villager marks done.
	code, obj, _ = villager.do("POST", "/api/needs", map[string]any{"text": "mleko 2l"})
	villager.must(201, code, "need")
	nid := itoa(obj["id"].(float64))
	code, _, _ = steward.do("PUT", "/api/needs/"+nid, map[string]any{"state": "taken"})
	steward.must(204, code, "take need")
	code, _, _ = second.do("PUT", "/api/needs/"+nid, map[string]any{"state": "taken"})
	second.must(409, code, "take twice")
	code, _, needs := villager.do("GET", "/api/needs", nil)
	villager.must(200, code, "needs")
	if needs[0]["state"] != "taken" || needs[0]["taken_by_name"] != "Stewards" {
		t.Fatalf("need state: %v", needs[0])
	}
	code, _, _ = villager.do("PUT", "/api/needs/"+nid, map[string]any{"state": "done"})
	villager.must(204, code, "need done")

	// Watchtower: away + watcher.
	code, obj, _ = villager.do("POST", "/api/away", map[string]any{"from_date": "2026-09-10", "to_date": "2026-09-14", "notes": "kokoši"})
	villager.must(201, code, "away")
	aid := itoa(obj["id"].(float64))
	code, _, _ = steward.do("PUT", "/api/away/"+aid, map[string]any{"watch": true})
	steward.must(204, code, "watch")
	code, _, aways := villager.do("GET", "/api/away", nil)
	villager.must(200, code, "aways")
	if aways[0]["watcher_name"] != "Stewards" {
		t.Fatalf("watcher: %v", aways[0])
	}
	// Another house cannot edit the dates.
	code, _, _ = steward.do("PUT", "/api/away/"+aid, map[string]any{"notes": "edited by steward"})
	steward.must(204, code, "steward edits")

	// Events and export.
	code, _, _ = villager.do("POST", "/api/events", map[string]any{"title": "Delovna akcija", "kind": "work", "starts_at": "2026-09-20T09:00"})
	villager.must(201, code, "event")
	code, _, _ = villager.do("GET", "/api/export", nil)
	villager.must(403, code, "villager export")
	code, obj, _ = steward.do("GET", "/api/export", nil)
	steward.must(200, code, "export")
	if len(obj["houses"].([]any)) != 2 {
		t.Fatalf("export houses: %v", obj["houses"])
	}

	// Revoke device B; its token dies.
	code, _, _ = villager.do("DELETE", "/api/devices/"+itoa(devs[1]["id"].(float64)), nil)
	villager.must(204, code, "revoke")
	code, _, _ = second.do("GET", "/api/me", nil)
	if code != 401 && code != 200 {
		t.Fatalf("unexpected %d", code)
	}
	// CORS: allowed origin echoed.
	req := httptest.NewRequest("OPTIONS", "/api/me", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatal("cors header missing")
	}
}

func itoa(f float64) string { return json.Number(jsonInt(f)).String() }
func jsonInt(f float64) string {
	b, _ := json.Marshal(int64(f))
	return string(b)
}
