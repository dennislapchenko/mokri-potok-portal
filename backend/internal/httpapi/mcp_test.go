package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// rpc sends one JSON-RPC request to /api/mcp with the given bearer and returns
// the HTTP status and the decoded envelope.
func rpc(t *testing.T, srv *Server, token, method string, params any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req := httptest.NewRequest("POST", "/api/mcp", &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// call runs one tool and returns isError and the text the tool answered.
func call(t *testing.T, srv *Server, token, tool string, args map[string]any) (bool, string) {
	t.Helper()
	code, out := rpc(t, srv, token, "tools/call", map[string]any{"name": tool, "arguments": args})
	if code != 200 {
		t.Fatalf("%s: http %d", tool, code)
	}
	if e, ok := out["error"].(map[string]any); ok {
		t.Fatalf("%s: rpc error %v", tool, e)
	}
	res := out["result"].(map[string]any)
	text := res["content"].([]any)[0].(map[string]any)["text"].(string)
	isErr, _ := res["isError"].(bool)
	return isErr, text
}

func mintKey(t *testing.T, c *client, label string) (string, string) {
	t.Helper()
	code, obj, _ := c.do("POST", "/api/devices/agent", map[string]any{"label": label})
	c.must(201, code, "mint key")
	tok := obj["token"].(string)
	if !strings.HasPrefix(tok, "potok_") || len(tok) != len("potok_")+64 {
		t.Fatalf("key shape: %q", tok)
	}
	return tok, itoa(obj["id"].(float64))
}

// TestMCPFlow: a house mints a key, the agent initialises, lists the tools,
// creates an event, and the phone sees it with the house's own yes on it.
func TestMCPFlow(t *testing.T) {
	srv, _, _, volk := newVillage(t)
	key, _ := mintKey(t, volk, "Claude on a laptop")

	code, out := rpc(t, srv, key, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "t"}})
	if code != 200 || out["result"].(map[string]any)["protocolVersion"] != "2025-06-18" {
		t.Fatalf("initialize: %d %v", code, out)
	}
	// An older client is answered in its own version; an unknown one gets ours.
	_, out = rpc(t, srv, key, "initialize", map[string]any{"protocolVersion": "2025-03-26"})
	if out["result"].(map[string]any)["protocolVersion"] != "2025-03-26" {
		t.Fatalf("older client: %v", out)
	}
	_, out = rpc(t, srv, key, "initialize", map[string]any{"protocolVersion": "1999-01-01"})
	if out["result"].(map[string]any)["protocolVersion"] != "2025-06-18" {
		t.Fatalf("unknown client version: %v", out)
	}
	// A notification is a 202 with no body.
	var buf bytes.Buffer
	buf.WriteString(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	req := httptest.NewRequest("POST", "/api/mcp", &buf)
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 202 || rec.Body.Len() != 0 {
		t.Fatalf("notification: %d %q", rec.Code, rec.Body.String())
	}
	_, out = rpc(t, srv, key, "ping", nil)
	if out["result"] == nil {
		t.Fatalf("ping: %v", out)
	}

	_, out = rpc(t, srv, key, "tools/list", nil)
	tools := out["result"].(map[string]any)["tools"].([]any)
	if len(tools) != len(mcpTools) {
		t.Fatalf("tools/list: %d of %d", len(tools), len(mcpTools))
	}

	isErr, text := call(t, srv, key, "whoami", nil)
	if isErr || !strings.Contains(text, `"name":"Zeleni Volk"`) {
		t.Fatalf("whoami: %v %s", isErr, text)
	}
	isErr, text = call(t, srv, key, "create_event", map[string]any{"title": "Košnja", "kind": "work", "starts_at": "2026-09-12T09:00", "place": "travnik"})
	if isErr {
		t.Fatalf("create_event: %s", text)
	}
	_, _, events := volk.do("GET", "/api/events", nil)
	if len(events) != 1 || events[0]["title"] != "Košnja" {
		t.Fatalf("event not in the calendar: %v", events)
	}
	var list []map[string]any
	json.Unmarshal([]byte(events[0]["signup_list"].(string)), &list)
	if answerOf(t, list, "Zeleni Volk")["state"] != "yes" {
		t.Fatalf("the caller's yes is missing: %v", list)
	}
	// A refused write comes back as the handler's own words, not a stack.
	isErr, text = call(t, srv, key, "create_event", map[string]any{"title": "x", "starts_at": "2026-09-12T09:00", "ends_at": "2026-09-11T09:00"})
	if !isErr || !strings.Contains(text, "ends_at is before starts_at") {
		t.Fatalf("bad range through MCP: %v %s", isErr, text)
	}
	// A 204 reads as ok.
	isErr, text = call(t, srv, key, "answer_event", map[string]any{"id": events[0]["id"], "state": "maybe"})
	if isErr || text != `{"ok":true}` {
		t.Fatalf("answer_event: %v %s", isErr, text)
	}
	// The key shows in the house's devices as an agent, and the phone does not.
	_, _, devs := volk.do("GET", "/api/devices", nil)
	agents := 0
	for _, d := range devs {
		if d["agent"].(float64) == 1 {
			agents++
		}
	}
	if len(devs) != 2 || agents != 1 {
		t.Fatalf("devices: %v", devs)
	}
}

// TestMCPFence: a key opens /api/mcp and nothing else. Every other route is a
// 403 with the key, including the ones that would let an agent lock its own
// house out or mint more keys; a steward's key is no steward.
func TestMCPFence(t *testing.T) {
	srv, _, steward, volk := newVillage(t)
	key, keyID := mintKey(t, volk, "agent")
	agent := &client{t: t, h: srv.Handler(), token: key}
	for _, rt := range []struct{ method, path string }{
		{"GET", "/api/me"}, {"GET", "/api/events"}, {"GET", "/api/away"}, {"GET", "/api/camp"},
		{"DELETE", "/api/devices/" + keyID}, {"GET", "/api/devices"}, {"POST", "/api/pair"},
		{"POST", "/api/devices/agent"}, {"PUT", "/api/houses/2"}, {"POST", "/api/push/subscribe"},
		{"PUT", "/api/me/device"}, {"DELETE", "/api/events/1"}, {"GET", "/api/export"},
	} {
		code, obj, _ := agent.do(rt.method, rt.path, map[string]any{})
		if code != 403 || obj["error"] != "this key speaks MCP only" {
			t.Fatalf("%s %s with a key: %d %v", rt.method, rt.path, code, obj)
		}
	}
	// The house's own phone still works, and the key did not touch it.
	code, _, devs := volk.do("GET", "/api/devices", nil)
	volk.must(200, code, "phone still in")
	if len(devs) != 2 {
		t.Fatalf("devices after the fence test: %v", devs)
	}
	// GET /api/mcp is not a stream.
	code, _, _ = agent.do("GET", "/api/mcp", nil)
	agent.must(405, code, "GET /api/mcp")

	// A steward's key is no steward: the fence answers first, and behind it
	// IsSteward is false — pinned by a steward-only handler reached in-process.
	skey, _ := mintKey(t, steward, "steward agent")
	sagent := &client{t: t, h: srv.Handler(), token: skey}
	for _, rt := range []struct{ method, path string }{{"POST", "/api/houses"}, {"GET", "/api/export"}, {"POST", "/api/houses/2/invite"}} {
		code, _, _ := sagent.do(rt.method, rt.path, map[string]any{"name": "x"})
		if code != 403 {
			t.Fatalf("steward key on %s %s: %d", rt.method, rt.path, code)
		}
	}
	// The other half of the fence: a phone's own session token is refused at
	// the door, so a steward cannot reach ownerOrSteward through MCP by
	// pasting what its browser holds.
	for _, c := range []*client{steward, volk} {
		code, obj, _ := c.do("POST", "/api/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
		if code != 403 || obj["error"] != "this door takes an MCP key, not a phone's token" {
			t.Fatalf("phone token on /api/mcp: %d %v", code, obj)
		}
	}
	// An unclean path segment is refused as invalid params, before the mux can
	// answer a bodiless 301 that would read back as a landed write.
	for _, bad := range []any{"..", ".", "1/2", "1?x=1", ""} {
		_, out := rpc(t, srv, key, "tools/call", map[string]any{"name": "update_event", "arguments": map[string]any{"id": bad, "title": "x"}})
		e, ok := out["error"].(map[string]any)
		if !ok || e["message"] != "bad id" {
			t.Fatalf("id %v: %v", bad, out)
		}
	}
	// And whoami says so: an agent reads its authority from that answer before
	// it does anything, so `is_steward` must be the request's, not the row's.
	isSteward, who := call(t, srv, skey, "whoami", map[string]any{})
	if isSteward || strings.Contains(who, `"is_steward":1`) {
		t.Fatalf("whoami tells a steward's key it is a steward: %s", who)
	}
	// The steward's own phone still reads as one.
	code, me, _ := steward.do("GET", "/api/me", nil)
	steward.must(200, code, "steward phone whoami")
	if me["is_steward"] != float64(1) {
		t.Fatalf("the steward's phone lost its steward flag: %v", me)
	}
	// In-process too: ownerOrSteward lets a steward edit any project, and the
	// steward's key is told "not yours" like any other house.
	_, obj, _ := volk.do("POST", "/api/projects", map[string]any{"title": "Ograja"})
	isErr, text := call(t, srv, skey, "update_project", map[string]any{"id": obj["id"], "state": "open"})
	if !isErr || !strings.Contains(text, "not yours") {
		t.Fatalf("steward key edits another house's project: %v %s", isErr, text)
	}
	code, _, _ = steward.do("PUT", "/api/projects/"+itoa(obj["id"].(float64)), map[string]any{"state": "open"})
	steward.must(204, code, "the steward's phone still may")
	// The fence is a context mark, not a header: a forged header changes nothing.
	req := httptest.NewRequest("GET", "/api/events", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-Via-MCP", "1")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Fatalf("forged mark: %d", rec.Code)
	}
}

// TestMCPSurface pins the tool table the way banner_test.go pins copy: no
// delete, no device, pairing, push or house write, no steward route; and every
// tool reaches a registered route.
func TestMCPSurface(t *testing.T) {
	srv, _, _, volk := newVillage(t)
	key, _ := mintKey(t, volk, "agent")
	want := map[string]bool{"list_away": true, "create_away": true, "update_away": true, "list_camp": true, "create_camp": true, "update_camp": true, "add_project_photo": true, "whoami": true, "weather": true}
	seen := map[string]bool{}
	for _, tl := range mcpTools {
		seen[tl.Name] = true
		if strings.HasPrefix(tl.Name, "delete") || tl.Method == "DELETE" {
			t.Fatalf("a delete on the surface: %s", tl.Name)
		}
		for _, forbidden := range []string{"/api/devices", "/api/pair", "/api/push", "/api/me/", "/api/export", "/api/houses/", "/api/bootstrap", "/api/join", "/api/prefs"} {
			if strings.HasPrefix(tl.Path, forbidden) {
				t.Fatalf("%s reaches %s", tl.Name, tl.Path)
			}
		}
		if tl.Path == "/api/houses" && tl.Method != "GET" {
			t.Fatalf("%s writes houses", tl.Name)
		}
		if strings.Contains(tl.Path, "/photo") && !tl.Image {
			t.Fatalf("%s carries bytes without the image flag", tl.Name)
		}
		// Every tool lands on a handler: never the /api/ catch-all, never 405.
		args := map[string]any{"id": 1, "project_id": 1, "subject": "event", "data": "AAAA", "content_type": "image/png"}
		isErr, text := call(t, srv, key, tl.Name, args)
		if strings.Contains(text, "no such endpoint") || (isErr && strings.Contains(text, "405")) {
			t.Fatalf("%s reaches no route: %s", tl.Name, text)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Fatalf("tool missing from the table: %s", name)
		}
	}
	if len(mcpTools) != 43 {
		t.Fatalf("the surface moved: %d tools — a visible decision, update this pin and docs/design-mcp.md", len(mcpTools))
	}
	// Unknown tool, unknown method, a batch, garbage.
	_, out := rpc(t, srv, key, "tools/call", map[string]any{"name": "delete_house", "arguments": map[string]any{}})
	if out["error"] == nil {
		t.Fatalf("unknown tool answered: %v", out)
	}
	_, out = rpc(t, srv, key, "resources/list", nil)
	if e := out["error"].(map[string]any); e["code"].(float64) != -32601 {
		t.Fatalf("unknown method: %v", out)
	}
	for _, body := range []string{`[{"jsonrpc":"2.0","id":1,"method":"ping"}]`, `{not json`} {
		req := httptest.NewRequest("POST", "/api/mcp", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+key)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		var o map[string]any
		json.Unmarshal(rec.Body.Bytes(), &o)
		if o["error"] == nil {
			t.Fatalf("accepted %q: %v", body, o)
		}
	}
}

// TestMCPKeyLifecycle: a common place cannot mint a key, a key cannot mint a
// key, and Remove turns the key into a 401 on /api/mcp.
func TestMCPKeyLifecycle(t *testing.T) {
	srv, _, steward, volk := newVillage(t)
	// A common place never holds a token in the app; plant one to prove the
	// route would refuse it anyway.
	_, obj, _ := steward.do("POST", "/api/houses", map[string]any{"name": "Parking", "kind": "common"})
	tok := randomToken(32)
	srv.st.Exec(nil2(), `INSERT INTO devices(house_id, token_hash, label) VALUES (?,?,?)`, int64(obj["id"].(float64)), hashToken(tok), "planted")
	common := &client{t: t, h: srv.Handler(), token: tok}
	code, o, _ := common.do("POST", "/api/devices/agent", map[string]any{"label": "x"})
	if code != 400 || o["error"] != "land, not an account" {
		t.Fatalf("common place minted a key: %d %v", code, o)
	}

	key, keyID := mintKey(t, volk, "agent")
	agent := &client{t: t, h: srv.Handler(), token: key}
	code, _, _ = agent.do("POST", "/api/devices/agent", map[string]any{"label": "child"})
	agent.must(403, code, "key mints key")

	code, _, _ = volk.do("DELETE", "/api/devices/"+keyID, nil)
	volk.must(204, code, "remove key")
	code, _ = rpc(t, srv, key, "ping", nil)
	if code != 401 {
		t.Fatalf("removed key still speaks: %d", code)
	}
	code, _ = rpc(t, srv, "", "ping", nil)
	if code != 401 {
		t.Fatalf("no token: %d", code)
	}
}

// TestMCPWatchtowerAndCamp: the Watchtower and the Campground are on the
// surface (owner's decision 2026-09-09). An agent reads and writes an away
// thread, becomes a watcher, and walks a camper's stay.
func TestMCPWatchtowerAndCamp(t *testing.T) {
	srv, _, steward, volk := newVillage(t)
	key, _ := mintKey(t, volk, "agent")
	_, away, _ := steward.do("POST", "/api/away", map[string]any{"from_date": "2026-09-10", "to_date": "2026-09-14", "notes": "kokoši"})
	id := away["id"]

	isErr, text := call(t, srv, key, "list_away", nil)
	if isErr || !strings.Contains(text, "kokoši") {
		t.Fatalf("list_away: %v %s", isErr, text)
	}
	isErr, text = call(t, srv, key, "update_away", map[string]any{"id": id, "watch": true})
	if isErr {
		t.Fatalf("watch: %s", text)
	}
	_, _, rows := steward.do("GET", "/api/away", nil)
	if rows[0]["watcher_name"] != "Zeleni Volk" {
		t.Fatalf("watcher not set: %v", rows[0])
	}
	// Dates belong to the house that is away: a 403 in the handler's words.
	isErr, text = call(t, srv, key, "update_away", map[string]any{"id": id, "to_date": "2026-09-20"})
	if !isErr || !strings.Contains(text, "not yours") {
		t.Fatalf("editing another house's dates: %v %s", isErr, text)
	}
	isErr, text = call(t, srv, key, "comment", map[string]any{"subject": "away", "id": id, "body": "nahranjene"})
	if isErr {
		t.Fatalf("comment on away: %s", text)
	}
	isErr, text = call(t, srv, key, "get_thread", map[string]any{"subject": "away", "id": id})
	if isErr || !strings.Contains(text, "nahranjene") {
		t.Fatalf("get_thread away: %v %s", isErr, text)
	}
	isErr, text = call(t, srv, key, "create_away", map[string]any{"from_date": "2026-10-01", "to_date": "2026-10-03"})
	if isErr {
		t.Fatalf("create_away: %s", text)
	}

	isErr, text = call(t, srv, key, "create_camp", map[string]any{"from_who": "grey camper"})
	if isErr {
		t.Fatalf("create_camp: %s", text)
	}
	var made map[string]any
	json.Unmarshal([]byte(text), &made)
	isErr, text = call(t, srv, key, "update_camp", map[string]any{"id": made["id"], "claim": true})
	if isErr {
		t.Fatalf("claim: %s", text)
	}
	isErr, text = call(t, srv, key, "update_camp", map[string]any{"id": made["id"], "state": "handed"})
	if isErr {
		t.Fatalf("handed: %s", text)
	}
	isErr, text = call(t, srv, key, "list_camp", nil)
	if isErr || !strings.Contains(text, `"state":"handed"`) || strings.Contains(text, "amount") {
		t.Fatalf("list_camp: %v %s", isErr, text)
	}
}

// TestMCPProjectPhoto: an agent puts a small picture on a project through the
// same handler and cap as the app; an oversized one gets the handler's 413.
func TestMCPProjectPhoto(t *testing.T) {
	srv, _, _, volk := newVillage(t)
	key, _ := mintKey(t, volk, "agent")
	isErr, text := call(t, srv, key, "create_project", map[string]any{"title": "Ograja"})
	if isErr {
		t.Fatalf("create_project: %s", text)
	}
	var p map[string]any
	json.Unmarshal([]byte(text), &p)
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	for _, data := range []string{base64.StdEncoding.EncodeToString(png), base64.RawStdEncoding.EncodeToString(png)} {
		isErr, text = call(t, srv, key, "add_project_photo", map[string]any{"project_id": p["id"], "data": data, "content_type": "image/png"})
		if isErr {
			t.Fatalf("add_project_photo: %s", text)
		}
	}
	isErr, text = call(t, srv, key, "get_project", map[string]any{"id": p["id"]})
	var proj map[string]any
	json.Unmarshal([]byte(text), &proj)
	if isErr || len(proj["photos"].([]any)) != 2 {
		t.Fatalf("photos on the project: %v %s", isErr, text)
	}
	isErr, text = call(t, srv, key, "add_project_photo", map[string]any{"project_id": p["id"], "data": base64.StdEncoding.EncodeToString(make([]byte, 2<<20+1)), "content_type": "image/png"})
	if !isErr || !strings.Contains(text, "over 2 MB") {
		t.Fatalf("oversize: %v %s", isErr, text)
	}
	isErr, text = call(t, srv, key, "add_project_photo", map[string]any{"project_id": p["id"], "data": "AAAA", "content_type": "image/gif"})
	if !isErr || !strings.Contains(text, "jpeg, png or webp") {
		t.Fatalf("gif: %v %s", isErr, text)
	}
	// The app's phone sees the picture bytes; the key never can.
	agent := &client{t: t, h: srv.Handler(), token: key}
	code, _, _ := agent.do("GET", "/api/photos/1", nil)
	agent.must(403, code, "photo bytes with a key")
}
