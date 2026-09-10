package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// A door for agents: the portal's API as MCP tools, so a house can say "put
// Saturday's work party on the calendar" to its assistant and have it in the
// portal. One route, POST /api/mcp, JSON-RPC 2.0 over plain HTTP, no
// dependency, no second process.
//
// Three rules hold the whole thing together:
//
//   - A key is a device. devices.agent = 1, minted by POST /api/devices/agent,
//     listed in "Your devices" with 🤖, revoked with the Remove button that
//     already exists, gone when the house is deleted. The portal keeps its
//     hash and cannot show it twice.
//   - A key opens /api/mcp and nothing else. requireHouse (auth.go) answers 403
//     to an agent device on any other route unless the request carries the
//     viaMCP mark, and only the dispatcher below sets it. So the tool table is
//     the surface: what is not in it, a key cannot do.
//   - A tool call is the existing handler run in-process. Every check the app
//     has — owner or steward, badRange, the codex rev, the answer `yes` a
//     creator gets on its own event — runs unchanged, and every push fires as
//     it would for a phone. There is no second write path.
//
// Tool descriptions are English: a model reads them, not a villager, so t()
// does not apply — the one place in the repo where that is true.

// viaMCP marks a sub-request the dispatcher built. A context value, so nothing
// from outside the process can forge it.
type viaMCP struct{}

const mcpProtocol = "2025-06-18"

// mcpMaxBody: a project picture rides in as base64, so the envelope needs more
// than readJSON's 64 KiB — 2 MB of photo is ~2.7 MiB of text. readPhoto still
// caps the decoded bytes at 2 MB.
const mcpMaxBody = 3 << 20

// mcpTool: one row is one tool. Path placeholders are named after the argument
// that fills them ({id}, {project_id}, {subject}); every other argument is the
// JSON body, which the handler reads exactly as it reads a phone's.
type mcpTool struct {
	Name, Desc   string
	Method, Path string
	Schema       map[string]any
	Image        bool // body is base64 `data` with `content_type`, not JSON
}

// The two clocks, said once here and pointed at from every time field.
const clocks = "Times you send are local wall clock, YYYY-MM-DDTHH:MM (a date alone is YYYY-MM-DD). Timestamps you read such as created_at are UTC with a space and seconds — never compare the two directly."

func prop(typ, desc string) map[string]any { return map[string]any{"type": typ, "description": desc} }

func schema(required []string, props map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

var (
	idProp     = prop("integer", "Row id")
	authorProp = prop("string", "Optional. The name of the person, only when they dictated the words; otherwise leave empty and the post reads as the house, which is true.")
)

var mcpTools = []mcpTool{
	{Name: "whoami", Desc: "Which house this key acts as, and its crest, colour and kind. Call it first.", Method: "GET", Path: "/api/me", Schema: schema(nil, map[string]any{})},
	{Name: "list_houses", Desc: "Every house with crest, colour, parcels it holds and parcels it lives on. kind=common is a common place (event grounds, parking): land on the map, not an account.", Method: "GET", Path: "/api/houses", Schema: schema(nil, map[string]any{})},
	{Name: "weather", Desc: "The ARSO forecast for a town some kilometres from the village, cached 30 min. A forecast, not a measurement at the village — never a frost source.", Method: "GET", Path: "/api/weather", Schema: schema(nil, map[string]any{})},

	// Tavern
	{Name: "list_posts", Desc: "The message board: posts and replies, pinned first, newest first. " + clocks, Method: "GET", Path: "/api/posts", Schema: schema(nil, map[string]any{})},
	{Name: "post", Desc: "Write on the board as your house. Rings the other houses.", Method: "POST", Path: "/api/posts",
		Schema: schema([]string{"body"}, map[string]any{"body": prop("string", "The text"), "author": authorProp, "parent_id": prop("integer", "Optional. Reply to this post; one reply level.")})},
	{Name: "list_events", Desc: "Calendar events from 60 days back onwards, with the answers given (signup_list) and your own (mine). An answer is stale when the event's time moved after it was given. " + clocks, Method: "GET", Path: "/api/events", Schema: schema(nil, map[string]any{})},
	{Name: "create_event", Desc: "Put an event or a work party on the calendar. Your house answers yes for itself in the same request; change it with answer_event. Rings the other houses. " + clocks, Method: "POST", Path: "/api/events",
		Schema: schema([]string{"title", "starts_at"}, map[string]any{
			"title": prop("string", "Title"), "kind": prop("string", "event (default) or work — a work party"),
			"starts_at": prop("string", "Local wall clock YYYY-MM-DDTHH:MM"), "ends_at": prop("string", "Optional, same shape, not before starts_at"),
			"place": prop("string", "Optional"), "notes": prop("string", "Optional"),
			"project_id": prop("integer", "Optional. The project this event belongs to"), "task_id": prop("integer", "Optional. A task of that project")})},
	{Name: "update_event", Desc: "Edit an event. Any house may, and the event names the house that last edited it. Moving starts_at or ends_at marks every answer stale and tells the houses that answered. " + clocks, Method: "PUT", Path: "/api/events/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "title": prop("string", ""), "kind": prop("string", "event or work"), "starts_at": prop("string", "YYYY-MM-DDTHH:MM"), "ends_at": prop("string", "YYYY-MM-DDTHH:MM, or empty for none"), "place": prop("string", ""), "notes": prop("string", "")})},
	{Name: "answer_event", Desc: "Answer for your house: yes, no or maybe. Silence is the fourth answer and there is no tool to speak it. Only yes is counted, and it is a headcount for one day, never a score.", Method: "POST", Path: "/api/events/{id}/signup",
		Schema: schema([]string{"id", "state"}, map[string]any{"id": prop("integer", "Event id"), "state": prop("string", "yes, no or maybe")})},
	{Name: "get_thread", Desc: "The comments under an event, a wish, an away notice or a contact, oldest first.", Method: "GET", Path: "/api/threads/{subject}/{id}",
		Schema: schema([]string{"subject", "id"}, map[string]any{"subject": prop("string", "event, wish, away or contact"), "id": prop("integer", "The event, wish, away notice or contact id")})},
	{Name: "comment", Desc: "Write in a thread as your house. One reply level. The houses the thread concerns are told; on an away notice the push says only that something was said, and on a contact nothing is pushed at all. A contact's thread is about the number — whether it still works, who came last, what to ask for — never about the person, who is not in the village and cannot answer.", Method: "POST", Path: "/api/threads/{subject}/{id}",
		Schema: schema([]string{"subject", "id", "body"}, map[string]any{"subject": prop("string", "event, wish, away or contact"), "id": prop("integer", "The event, wish, away notice or contact id"), "body": prop("string", "The text"), "author": authorProp, "parent_id": prop("integer", "Optional. Reply to this comment")})},

	// Market
	{Name: "list_runs", Desc: "Shop runs whose cut-off is not more than a day past: who drives where, and until when a need can ride along. " + clocks, Method: "GET", Path: "/api/runs", Schema: schema(nil, map[string]any{})},
	{Name: "create_run", Desc: "Announce that your house drives somewhere and takes needs along. Rings the other houses. " + clocks, Method: "POST", Path: "/api/runs",
		Schema: schema([]string{"destination", "cutoff_at"}, map[string]any{"destination": prop("string", "Where, e.g. the town's name or a shop"), "cutoff_at": prop("string", "Last moment to add a need, YYYY-MM-DDTHH:MM"), "notes": prop("string", "Optional")})},
	{Name: "update_run", Desc: "Change your run (or any run, as a steward — but a key is never a steward). A changed destination or cut-off tells the houses whose open needs ride on it; a notes change tells nobody. " + clocks, Method: "PUT", Path: "/api/runs/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "destination": prop("string", ""), "cutoff_at": prop("string", "YYYY-MM-DDTHH:MM"), "notes": prop("string", "")})},
	{Name: "list_needs", Desc: "What houses need from the shop: open, taken (a house will bring it) or done in the last week, with the run it rides on.", Method: "GET", Path: "/api/needs", Schema: schema(nil, map[string]any{})},
	{Name: "create_need", Desc: "Ask for something from the shop, optionally on a run. Rings the other houses.", Method: "POST", Path: "/api/needs",
		Schema: schema([]string{"text"}, map[string]any{"text": prop("string", "What is needed"), "run_id": prop("integer", "Optional. The run it should ride on")})},
	{Name: "update_need", Desc: "state=taken: your house will bring it (any house, from open). state=open: let it go again (the taker, the owner). state=done: the owner or the taker closes it. text: the owner edits it. Taken by is an acknowledgment, never a tally.", Method: "PUT", Path: "/api/needs/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "state": prop("string", "taken, open or done"), "text": prop("string", "")})},
	{Name: "list_offers", Desc: "Give-aways, seeds, surplus and joint orders: open, claimed or done in the last week.", Method: "GET", Path: "/api/offers", Schema: schema(nil, map[string]any{})},
	{Name: "create_offer", Desc: "Offer something. tag: giveaway (default), seeds, surplus, or joint — a joint order where the app records the order and never the money. Rings the other houses.", Method: "POST", Path: "/api/offers",
		Schema: schema([]string{"text"}, map[string]any{"text": prop("string", "What is offered"), "tag": prop("string", "giveaway, seeds, surplus or joint")})},
	{Name: "update_offer", Desc: "state=claimed: your house takes it (any house, from open). state=open: release. state=done: the owner or the claimer closes it. text and tag: the owner edits them.", Method: "PUT", Path: "/api/offers/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "state": prop("string", "claimed, open or done"), "text": prop("string", ""), "tag": prop("string", "giveaway, seeds, surplus or joint")})},

	// Watchtower. Owner's decision 2026-09-09: a key is the logged-in house and
	// the model provider is a third party that house chose, like WhatsApp. An
	// agent reads a notice when its house asks; nothing here pushes or digests.
	{Name: "list_away", Desc: "Who is away and until when, with the care notes and the watcher, from three days back. This is burglary information about the houses of your village: read it only when your house asks, keep it inside this conversation, and never repeat it anywhere else.", Method: "GET", Path: "/api/away", Schema: schema(nil, map[string]any{})},
	{Name: "create_away", Desc: "Say your house is away from a date to a date, with a note for whoever watches the place. The push names no house, no dates and no notes. Dates are YYYY-MM-DD.", Method: "POST", Path: "/api/away",
		Schema: schema([]string{"from_date", "to_date"}, map[string]any{"from_date": prop("string", "YYYY-MM-DD"), "to_date": prop("string", "YYYY-MM-DD, not before from_date"), "notes": prop("string", "Optional. What needs watching: animals, water, a key")})},
	{Name: "update_away", Desc: "watch=true: your house becomes the watcher, if nobody is yet. watch=false: step back (the watcher). from_date, to_date, notes: the house that is away edits them.", Method: "PUT", Path: "/api/away/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "watch": prop("boolean", "Become the watcher (true) or step back (false)"), "from_date": prop("string", "YYYY-MM-DD"), "to_date": prop("string", "YYYY-MM-DD"), "notes": prop("string", "")})},

	// Tool shed
	{Name: "list_tools", Desc: "What the village lends: each tool with its owner house, its category (power, garden, other) and who holds it now. Never how many times anyone borrowed.", Method: "GET", Path: "/api/tools", Schema: schema(nil, map[string]any{})},
	{Name: "create_tool", Desc: "Put a tool of your house in the shed. Rings the other houses.", Method: "POST", Path: "/api/tools",
		Schema: schema([]string{"name"}, map[string]any{"name": prop("string", "The tool"), "notes": prop("string", "Optional, e.g. bring your own fuel"), "category": prop("string", "power, garden or other")})},
	{Name: "update_tool", Desc: "take=true: your house borrows it (if free; the owner is told). take=false: give it back. name, notes, category: the owner edits them.", Method: "PUT", Path: "/api/tools/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "take": prop("boolean", "Borrow (true) or return (false)"), "name": prop("string", ""), "notes": prop("string", ""), "category": prop("string", "power, garden or other")})},
	{Name: "list_wishes", Desc: "Tools the village lacks, with the names of the houses that would love one (wants — names, never a count), the options houses found, and whether yours is among them (mine).", Method: "GET", Path: "/api/wishes", Schema: schema(nil, map[string]any{})},
	{Name: "create_wish", Desc: "Wish for a tool the village lacks. Your house is on the list by definition.", Method: "POST", Path: "/api/wishes",
		Schema: schema([]string{"text"}, map[string]any{"text": prop("string", "The tool")})},
	{Name: "update_wish", Desc: "want=true puts your house's name under the wish, want=false takes it off. A name, never a vote.", Method: "PUT", Path: "/api/wishes/{id}",
		Schema: schema([]string{"id", "want"}, map[string]any{"id": idProp, "want": prop("boolean", "")})},
	{Name: "add_wish_option", Desc: "Add a finding to a wish: this model, this price, this link. Options are never counted, ranked or marked as a winner.", Method: "POST", Path: "/api/wishes/{id}/options",
		Schema: schema([]string{"id", "text"}, map[string]any{"id": prop("integer", "Wish id"), "text": prop("string", "What you found"), "url": prop("string", "Optional link")})},

	// Projects
	{Name: "list_projects", Desc: "Long jobs with their state (planned, open = in progress, done), task counts, picture and event counts and the next event. " + clocks, Method: "GET", Path: "/api/projects", Schema: schema(nil, map[string]any{})},
	{Name: "get_project", Desc: "One project with its tasks, events and picture list (ids and who added them; the bytes need the app).", Method: "GET", Path: "/api/projects/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp})},
	{Name: "create_project", Desc: "Start a long job. It begins planned; step it to open when work starts. Rings the other houses (\"plans\").", Method: "POST", Path: "/api/projects",
		Schema: schema([]string{"title"}, map[string]any{"title": prop("string", ""), "notes": prop("string", "Optional"), "due_at": prop("string", "Optional, YYYY-MM-DD")})},
	{Name: "update_project", Desc: "The creator edits title, notes, due_at and steps state: planned, open (in progress) or done. Done is a state, never a deletion.", Method: "PUT", Path: "/api/projects/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "title": prop("string", ""), "notes": prop("string", ""), "due_at": prop("string", "YYYY-MM-DD, or empty for none"), "state": prop("string", "planned, open or done")})},
	{Name: "create_task", Desc: "Add a task to a project. Any house may.", Method: "POST", Path: "/api/projects/{project_id}/tasks",
		Schema: schema([]string{"project_id", "title"}, map[string]any{"project_id": prop("integer", ""), "title": prop("string", ""), "notes": prop("string", "Optional"), "due_at": prop("string", "Optional, YYYY-MM-DD")})},
	{Name: "update_task", Desc: "take=true: your house takes a free task (the project's creator is told); take=false: let it go. state=done with closing_note closes it (the holder or the creator); state=open reopens. assigned_to hands it to a house — only the task's creator or the project's creator, after agreeing in person; that house is told. title, notes, due_at: the creator edits.", Method: "PUT", Path: "/api/tasks/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "take": prop("boolean", ""), "assigned_to": prop("integer", "House id"), "state": prop("string", "done or open"), "closing_note": prop("string", "With state=done"), "title": prop("string", ""), "notes": prop("string", ""), "due_at": prop("string", "YYYY-MM-DD")})},
	{Name: "add_project_photo", Desc: "Put a picture on a project, as your house. Raw image bytes, base64: jpeg, png or webp, at most 2 MB after decoding. Nothing shrinks it on this path — send something already small (the app shrinks to ~1000 px). There is no tool to take a picture down: the house that added it, the project's house or a steward does that in the app.", Method: "POST", Path: "/api/projects/{project_id}/photos", Image: true,
		Schema: schema([]string{"project_id", "data", "content_type"}, map[string]any{"project_id": prop("integer", ""), "data": prop("string", "The image, base64 (standard alphabet, with or without padding)"), "content_type": prop("string", "image/jpeg, image/png or image/webp")})},

	// Campground
	{Name: "list_camp", Desc: "Camper stays at the village parking: arrived (a house noticed), held (a house has the money), handed (it reached the box). One row is one stay. There is no amount and no total; the cash box is the ledger.", Method: "GET", Path: "/api/camp", Schema: schema(nil, map[string]any{})},
	{Name: "create_camp", Desc: "A camper arrived. One row is one camper's stay; there is no amount and no total. Do not put a licence plate or a nationality in the label — \"grey camper\" is enough. have_money=true means your house already has the cash, and the row lands as handed. Rings the other houses.", Method: "POST", Path: "/api/camp",
		Schema: schema(nil, map[string]any{"from_who": prop("string", "Optional label for the camper, e.g. grey camper — no plate, no nationality"), "notes": prop("string", "Optional"), "taken_on": prop("string", "Optional, YYYY-MM-DD; default today"), "have_money": prop("boolean", "Your house already holds the cash")})},
	{Name: "update_camp", Desc: "claim=true: your house has the money (from arrived; the others hear, so nobody collects twice). state=handed: it reached the box (the holder). state=held: back from handed (the holder). notes: the noticer or the holder edits.", Method: "PUT", Path: "/api/camp/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "claim": prop("boolean", ""), "state": prop("string", "handed or held"), "notes": prop("string", "")})},

	// Contacts: the village phone book. Owner's decision 2026-09-10, the same
	// line the Watchtower is on — a key is the logged-in house, and the model
	// provider is a third party that house chose. The numbers belong to people
	// who never joined the portal, so the descriptions say to keep them here.
	{Name: "list_contacts", Desc: "The village phone book: name, phone, notes and a type (vet, craftsman, office — a free word, not a fixed list), with the house that added each number and the house that last corrected it. These are the numbers of people outside the village who never agreed to this portal: read them when your house asks, keep them inside this conversation, and never repeat them anywhere else.", Method: "GET", Path: "/api/contacts", Schema: schema(nil, map[string]any{})},
	{Name: "create_contact", Desc: "Add a number to the phone book. Nothing is pushed — a phone book is looked up, not announced. type is one word: reuse one that list_contacts already shows, or write a new one and it exists from then on (a spelling that differs only in case is folded into the one already there).", Method: "POST", Path: "/api/contacts",
		Schema: schema([]string{"name"}, map[string]any{"name": prop("string", "Who or what it is, e.g. the well-driller's name or a workshop"), "phone": prop("string", "Optional. The number, as a person would write it down"), "notes": prop("string", "Optional, e.g. speaks German, comes on Tuesdays"), "type": prop("string", "Optional. One word: vet, craftsman, office …")})},
	{Name: "update_contact", Desc: "Correct a contact. Any house may correct any number — a wrong one helps nobody — and the row then names your house as the one that last wrote it. There is no tool to remove a contact: the house that added it, or a steward, does that in the app.", Method: "PUT", Path: "/api/contacts/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "name": prop("string", ""), "phone": prop("string", ""), "notes": prop("string", ""), "type": prop("string", "One word; empty leaves it untyped")})},

	// Codex
	{Name: "list_codex", Desc: "The village's founding text as ordered bilingual sections (title_sl/en, body_sl/en), each with its rev and the house that last wrote it; updated_by null means the text stands as the council adopted it. Bodies are plain text: a blank line splits paragraphs, lines starting with \"- \" are a list.", Method: "GET", Path: "/api/codex", Schema: schema(nil, map[string]any{})},
	{Name: "create_codex_section", Desc: "Append a section, in your house's name. At least one title. Nothing pushes: announce it in the Tavern in your house's words.", Method: "POST", Path: "/api/codex",
		Schema: schema(nil, map[string]any{"title_sl": prop("string", ""), "title_en": prop("string", ""), "body_sl": prop("string", ""), "body_en": prop("string", "")})},
	{Name: "update_codex_section", Desc: "Edit a section; it then names your house and today. Send the rev you read: a 409 means another house wrote first — read again, then decide. Without rev the write lands as it is.", Method: "PUT", Path: "/api/codex/{id}",
		Schema: schema([]string{"id"}, map[string]any{"id": idProp, "rev": prop("integer", "The rev from list_codex"), "title_sl": prop("string", ""), "title_en": prop("string", ""), "body_sl": prop("string", ""), "body_en": prop("string", "")})},
}

// ---- the key --------------------------------------------------------------

// createAgentDevice mints a key: a device row with agent=1 and a token the
// house sees once. `potok_` in front so a human who finds it in a config file
// knows what it opens; the hash covers the whole string.
func (s *Server) createAgentDevice(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	if h.Agent {
		writeErr(w, 403, "a key does not mint keys")
		return
	}
	// A common place has no login and must not grow one.
	if !s.accountRow(w, r, h.ID) {
		return
	}
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	tok := "potok_" + randomToken(32)
	id, err := s.st.Exec(r.Context(), `INSERT INTO devices(house_id, token_hash, label, agent) VALUES (?,?,?,1)`, h.ID, hashToken(tok), str(m, "label"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "token": tok})
}

// ---- JSON-RPC -------------------------------------------------------------

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func rpcReply(w http.ResponseWriter, id json.RawMessage, result any, err *rpcError) {
	out := map[string]any{"jsonrpc": "2.0", "id": id}
	if err != nil {
		out["error"] = err
	} else {
		out["result"] = result
	}
	if id == nil {
		out["id"] = nil
	}
	writeJSON(w, 200, out)
}

// mcp is the endpoint: one JSON-RPC request per POST, answered as JSON. No
// SSE, no session id (a stateless server may issue none), no batches (the
// 2025-06-18 revision dropped them).
func (s *Server) mcp(w http.ResponseWriter, r *http.Request) {
	// A phone's session token is refused here, and that is the other half of
	// "a key carries no stewardship": the fence in requireHouse keeps a key
	// out of every other route, and this keeps a full login out of this one.
	// Without it a steward could paste the token its browser holds into an
	// agent and edit every house's rows through ownerOrSteward — the decision
	// would have a side door nobody wrote down.
	if !houseFrom(r).Agent {
		writeErr(w, http.StatusForbidden, "this door takes an MCP key, not a phone's token")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, mcpMaxBody))
	if err != nil {
		rpcReply(w, nil, nil, &rpcError{-32600, "body too large"})
		return
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		rpcReply(w, nil, nil, &rpcError{-32600, "batches are not supported; send one request per POST"})
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(trimmed, &req); err != nil {
		rpcReply(w, nil, nil, &rpcError{-32700, "parse error"})
		return
	}
	// A notification has no id and expects no body.
	if strings.HasPrefix(req.Method, "notifications/") {
		w.WriteHeader(202)
		return
	}
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &p)
		version := mcpProtocol
		if p.ProtocolVersion == "2025-03-26" {
			version = p.ProtocolVersion
		}
		rpcReply(w, req.ID, map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "mokri-potok-portal", "version": "1"},
			"instructions": "You act as one house of a small village portal, with that house's name on everything you write. Call whoami first. " + clocks +
				" Leave `author` empty unless the person dictated the words. Away notices are burglary information: read them only when asked and keep them in this conversation.",
		}, nil)
	case "ping":
		rpcReply(w, req.ID, map[string]any{}, nil)
	case "tools/list":
		list := make([]map[string]any, 0, len(mcpTools))
		for _, t := range mcpTools {
			list = append(list, map[string]any{"name": t.Name, "description": t.Desc, "inputSchema": t.Schema})
		}
		rpcReply(w, req.ID, map[string]any{"tools": list}, nil)
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			rpcReply(w, req.ID, nil, &rpcError{-32602, "bad params"})
			return
		}
		tool := mcpToolByName(p.Name)
		if tool == nil {
			rpcReply(w, req.ID, nil, &rpcError{-32602, "no such tool: " + p.Name})
			return
		}
		status, body, msg := s.dispatch(r, tool, p.Arguments)
		if msg != "" {
			rpcReply(w, req.ID, nil, &rpcError{-32602, msg})
			return
		}
		rpcReply(w, req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": body}},
			// A 3xx is not a landed write: Go's mux answers an unclean path
			// with a bodiless 301, which would otherwise read as {"ok":true}.
			// Belt with no test on it — the path filter in dispatch refuses
			// `.` and `..` first, so nothing here can currently reach a 301.
			// Keep both: the filter is what a new tool's path could slip past.
			"isError": status >= 300,
		}, nil)
	default:
		rpcReply(w, req.ID, nil, &rpcError{-32601, "method not found: " + req.Method})
	}
}

func mcpToolByName(name string) *mcpTool {
	for i := range mcpTools {
		if mcpTools[i].Name == name {
			return &mcpTools[i]
		}
	}
	return nil
}

// dispatch builds the HTTP request a phone would have sent and runs it through
// the mux in-process, with the caller's bearer and the viaMCP mark. The handler
// that answers is the one the phone talks to. It returns the status, the body
// as text, and a message when the arguments could not even form a request.
func (s *Server) dispatch(r *http.Request, t *mcpTool, args map[string]any) (int, string, string) {
	if args == nil {
		args = map[string]any{}
	}
	body := map[string]any{}
	path := t.Path
	for k, v := range args {
		if strings.Contains(path, "{"+k+"}") {
			seg := argString(v)
			if seg == "" || seg == "." || seg == ".." || strings.ContainsAny(seg, "/?#%") {
				return 0, "", "bad " + k
			}
			path = strings.ReplaceAll(path, "{"+k+"}", seg)
			continue
		}
		body[k] = v
	}
	if strings.Contains(path, "{") {
		return 0, "", "missing " + path[strings.Index(path, "{")+1:strings.Index(path, "}")]
	}
	var reader io.Reader
	ct := ""
	switch {
	case t.Image:
		data, _ := body["data"].(string)
		// Padding stripped first, so a client that pads and one that does not
		// both land in the raw decoder.
		raw, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(data, "="))
		if err != nil {
			return 0, "", "data is not base64"
		}
		reader = bytes.NewReader(raw)
		ct, _ = body["content_type"].(string)
	case t.Method != "GET":
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
		ct = "application/json"
	}
	sub, err := http.NewRequestWithContext(context.WithValue(r.Context(), viaMCP{}, true), t.Method, path, reader)
	if err != nil {
		return 0, "", err.Error()
	}
	sub.Header.Set("Authorization", r.Header.Get("Authorization"))
	if ct != "" {
		sub.Header.Set("Content-Type", ct)
	}
	sub.RemoteAddr = r.RemoteAddr
	rec := &memResponse{header: http.Header{}, status: 200}
	s.Handler().ServeHTTP(rec, sub)
	text := strings.TrimSpace(rec.body.String())
	if text == "" {
		text = `{"ok":true}`
	}
	return rec.status, text, ""
}

// argString: a path segment from a JSON argument. A number arrives as float64
// and must print as an integer, never as 1e+06.
func argString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}

// memResponse is the fifteen-line ResponseWriter the dispatcher writes into.
type memResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (m *memResponse) Header() http.Header         { return m.header }
func (m *memResponse) WriteHeader(code int)        { m.status = code }
func (m *memResponse) Write(b []byte) (int, error) { return m.body.Write(b) }
