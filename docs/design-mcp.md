# A door for agents: the portal's API as MCP

Decided and built 2026-09-09. The first half is the argument and what exists;
the appendix pins what the code does so nobody re-derives it. The invariants
live in `CLAUDE.md`; this file is the rationale and the options not taken.

## What it is for

A house that works with an agent — the owner's, and a villager's the same —
says "put Saturday's work party on the calendar", "what does the village need
from the shop", "read the codex section on animals", and it is in the portal.
Today that is a phone in the hand and five taps. The MCP server lets the agent
do it through the portal's own API, **as that house** — the same rows, the
same pushes, the same rules. A house that runs no agent loses nothing: no room
changes, no bar changes.

The villager job, in `village.md`: *"tell my assistant, and it is in the
portal."*

## Answer first

- **One more route in the Go binary: `POST /api/mcp`, MCP over plain HTTP,
  JSON-RPC written by hand** (`mcp.go`). Five methods, no dependency, no second
  process, same origin, bearer token like every other route.
- **A tool is a row in a table, and a call is the existing HTTP handler run
  in-process.** Every check the app has — owner or steward, `badRange`, the
  colour regex, the codex `rev` — runs unchanged, and every push fires as it
  would for a phone. No second write path.
- **A key is a device.** `devices.agent = 1`, one column. The token is the
  same 32 random bytes a phone gets, with `potok_` in front, hashed the same
  way, minted once by `POST /api/devices/agent`, listed in "Your devices" with
  🤖, revoked with the "Remove" button that already exists. No new table.
- **A key opens `/api/mcp` and nothing else.** `requireHouse` answers 403 to
  an agent device on any other route; the tools reach the handlers through the
  dispatcher, which marks its sub-requests. So **the tool table is the whole
  surface** — not a menu in front of a full login — and `mcp_test.go` pins it.
- **Shown once.** The portal stores the hash, so it *cannot* show the key
  again; the box under the button says so and offers Copy.
- **A key carries no stewardship for now**, whatever house made it. The owner
  left the door open: a key might carry it later. Not a closed question.
- **The surface: every room.** Tavern, Market, Watchtower, Shed, Projects with
  pictures, Campground, Codex, Contacts, weather. Reads and writes, **no
  deletes**.
- The button sits at the foot of the Houses room, right of *Log out on this
  device*: **`+ MCP key`**. Plain weight.

## Where the server lives

| Option | What it costs | Verdict |
| --- | --- | --- |
| A. A route in the Go binary, plain HTTP, JSON-RPC by hand | One Go file, `mcp.go`, most of it the tool table. Zero deps. One origin, one container, one Caddy entry that already exists. The agent's laptop needs nothing installed | **Built** |
| B. A stdio shim in the repo (Node or Python) that calls the HTTP API | A second language in the repo, a runtime on every laptop, and the key in a local file anyway. Buys nothing: Claude Code, Codex and Cursor speak HTTP MCP natively today | No. A client that only speaks stdio bridges it on its own side; that is the client's business |
| C. The official Go MCP SDK | A dependency for five JSON-RPC methods. `CLAUDE.md` says adding one is a decision to write down; the arithmetic here is a few hundred lines against a module tree | No |
| D. `server mcp` subcommand on stdio inside the container | Reachable only by `docker exec`; SSH to the VM would be the credential for every agent call | No |

What A costs: **the MCP spec's authorization for HTTP servers is OAuth 2.1**,
and a static bearer is what the clients accept as a configured header
instead. Claude Code takes one (`--header`); the endpoint was exercised with
curl carrying that header, not with any client yet. Other clients whose
config takes a header should work the same way — untried. **claude.ai's web
connectors expect OAuth and will not connect.** OAuth is out of scope; a row
in the Decisions table records it.

Nothing else moved: the CSP is on `index.html` only and `/api/mcp` is not a
page; the whole host is already proxied to the container, so Caddy in the
gaias-choice repo did not change; `.doco-cd.yml` did not change.

## The key

| | Invite | Pairing code | MCP key |
| --- | --- | --- | --- |
| Shape | 10 chars, phone-typable | 6 digits | `potok_` + 64 hex (32 random bytes), pasted, never typed |
| Who makes it | Steward | The house, from a phone inside | The house, from a phone inside |
| Lives | 14 days, multi-use | 15 minutes, single use | Until removed, like a phone's token |
| Guessing | Per-IP cap on `/api/join` | Same cap, plus five misses burn every live code | Not a code: 256 bits, resolved by `requireHouse` like every device token. **The `/api/join` arithmetic is untouched** |
| Stored as | Plain, it *is* the link | Plain, burned on use | SHA-256, like a device token |
| Reaches | The gate, once | The gate, once | **`/api/mcp` only**; 403 everywhere else |
| Ends | Expiry, or ♻ | Use, expiry, or the burn | The "Remove" button in Your devices, or deleting the house |

The `potok_` prefix is one line and buys recognisability: a human who finds
the string in a config file knows what it opens. The hash covers the whole
string.

**Why a device row and not an `api_tokens` table.** `requireHouse` already
resolves a device token to a house in one query; "Your devices" already
lists, labels and revokes; `last_seen` already says when it last acted (for
an agent that is the useful line: "last used Tuesday"); deleting a house
already cascades. A second table is a second lookup, a second list, a
second delete, and the same `agent` flag anyway.

**Why the key opens only `/api/mcp`.** Without that line, a key is a full
house login with a menu in front of it: `curl` with the same bearer reaches
`DELETE /api/events/{id}`, `POST /api/pair`, and `DELETE /api/devices/{id}` —
the last one lets an agent in a loop remove every phone of its own house and
lock it out until a steward rotates the invite. One check in `requireHouse`
closes all of it: an agent device is allowed through only when the request
carries the dispatcher's mark, and `POST /api/mcp` is the one route that does
not need the mark. The cost: an agent cannot use the raw REST API with its
key. That is the point — "the same way a user can" means the same handlers,
not the same wire. The alternative was a denylist of routes with the same 403
— more rows, the same test — and the owner chose the fence.

**No expiry**, like a phone. An expiring key fails silently on a laptop months
later, and the fix is the same button.

**Stewardship.** `requireHouse` reads `d.agent` and sets `IsSteward = false`
when it is 1. The steward routes — create or delete a house, rotate an
invite, toggle a steward, mute a kind village-wide, delete a codex section,
export — are exactly the ones the UI puts a confirm dialog or a steward-only
block in front of, and an agent has neither. The cost: the owner's own agent
cannot export or create a house. The owner: fine for now, and a key might
carry stewardship later. Flipping it is one line.

**Nothing marks a row as written by an agent.** A post from a key is the
house's post, like a post from any of its phones — the house answers for its
key as for its phones. `edited_by` on an event names the house, as today.
Recording `device_id` on rows was considered and refused: it is a step
toward per-person accounts, which needs a recorded decision.

**What a bot signs.** The optional `author` on a post or a comment is "the
name the poster typed themselves", and a neighbour who reads *Ana · 🏠 …*
answers a person. An agent passing the person's name breaks that; an agent
passing nothing reads as the house, like a post from a phone with no name,
which is true. So the tool descriptions, the `initialize` instructions and
the README all say: **leave `author` empty unless the person dictated the
words.**

**No rate limit.** Authenticated routes have none today, and an agent in a
loop is a house posting in a loop: fifty events are fifty pushes to every
phone. The brake is Remove, and the key is one tap from it. Accepted; written
here so nobody adds a counter later without seeing the invariant it brushes.

## The tool surface

Reads and writes an ordinary house has in every room, **no deletes** (done is
a state, and an agent that deletes has no confirm), no devices, pairing or
push (a key must not mint keys), no `PUT /api/houses/{id}` (rename, crest,
colour, the map mark: a house does that itself, once), no steward route. With
the fence, this table is not a menu: **what is not here, a key cannot do**.
46 tools; `TestMCPSurface` pins the count so a change is a visible decision.

| Tool | Route | Notes the description carries |
| --- | --- | --- |
| `whoami` | `GET /api/me` | First thing an agent calls: which house am I |
| `list_houses` | `GET /api/houses` | Crest, colour, parcels, homes. Common places included, marked |
| `weather` | `GET /api/weather` | ARSO, a town away, never a frost source |
| `list_posts` · `post` | `GET`/`POST /api/posts` | `body`, optional `author`, optional `parent_id` |
| `list_events` · `create_event` · `update_event` | `GET`/`POST /api/events`, `PUT /api/events/{id}` | `kind` is `event` or `work`; `starts_at` wall clock `YYYY-MM-DDTHH:MM`; creating answers `yes` for your house; moving the time marks every answer stale |
| `answer_event` | `POST /api/events/{id}/signup` | `state`: `yes` / `no` / `maybe`; silence is the fourth thing, and there is no tool to speak it |
| `get_thread` · `comment` | `GET`/`POST /api/threads/{subject}/{id}` | `subject`: `event`, `wish`, `away` or `contact`; one reply level. A contact's thread pushes to nobody, because the phone book has no push kind. The away thread is on the surface with the Watchtower, and `TestMCPWatchtowerAndCamp` pins that an agent reads and writes it |
| `list_runs` · `create_run` · `update_run` | `/api/runs` | `destination`, `cutoff_at`; moving a run pushes to its riders |
| `list_needs` · `create_need` · `update_need` | `/api/needs` | `text`, optional `run_id`; `state` open / taken / done |
| `list_offers` · `create_offer` · `update_offer` | `/api/offers` | `tag`: giveaway / seeds / surplus / joint |
| `list_away` · `create_away` · `update_away` | `/api/away` | Dates `YYYY-MM-DD`; `watch: true/false` is the watcher; the description says it is burglary information, read it only when asked and keep it in the conversation |
| `list_tools` · `create_tool` · `update_tool` | `/api/tools` | `take: true/false` is the loan; a tool comes back |
| `list_wishes` · `create_wish` · `update_wish` · `add_wish_option` | `/api/wishes`, `/api/wishes/{id}/options` | `want: true/false` is a name on the list, never a count; an option is a finding |
| `list_projects` · `get_project` · `create_project` · `update_project` | `/api/projects` | A new project begins `planned`; `state` steps it |
| `create_task` · `update_task` | `POST /api/projects/{project_id}/tasks`, `PUT /api/tasks/{id}` | `take`, `state: done` with `closing_note`; `assigned_to` only if you created the task or the project |
| `add_project_photo` | `POST /api/projects/{project_id}/photos` | `data` base64 + `content_type`; the handler's own 2 MB cap and 415/413 pass through; no browser-side shrink on this path, so send something small; **no delete** — a house or a steward takes a picture down in the app |
| `list_camp` · `create_camp` · `update_camp` | `/api/camp` | One row is one camper's stay; no amount, no total, no plate or nationality in the label — the handler refuses nothing here, so the description carries the rule |
| `list_codex` · `create_codex_section` · `update_codex_section` | `/api/codex` | Send the `rev` you read; a 409 means another house wrote first |
| `list_contacts` · `create_contact` · `update_contact` | `/api/contacts` | The village phone book. `type` is a free word, not a fixed list; a spelling differing only in case folds into the one in use. The numbers belong to people outside the village who never agreed to this portal, so the description says to keep them in the conversation, as `list_away`'s does. Nothing here pushes |

Off the surface, each with its argument:

- **Deletes** — an agent deleting an event deletes its sign-ups and thread;
  a phone gets a confirm first. If the owner wants them, they come as one
  `delete_*` per table with the same owner-or-steward check the route has.
- **Tool photos and reading picture bytes** — a tool photo is the owner's,
  set once from the shed; picture bytes are what the app shows. An agent
  gets the picture *list* on a project (who added, when), never the bytes.
- **Devices, pairing, push, the house's own row, every steward route** — a
  key must not mint keys or lock its house out, and stewardship is a phone's
  act behind a confirm dialog.

Every description that takes or returns a time carries the two clocks, and
so do the `initialize` instructions: a time the tool sends is local wall
clock `YYYY-MM-DDTHH:MM`; a `created_at` it reads is UTC with a space. An
agent that compares them directly gets the village two hours wrong in
summer, same as the page would.

### The Watchtower and the Campground, and the invariant they touched

`CLAUDE.md` § Nothing is public says away-notices "never leave the logged-in
app (no digests, no feeds)". The question put to the owner: is a model
provider's context window a new kind of leaving? The owner's line, 2026-09-09:
**a key is the logged-in house, and the model provider is a third party that
house chose** — the same argument that lets an away-notice sit in a WhatsApp
group. So the Watchtower is on the surface. The "no digests, no feeds" half
stands untouched: nothing pushes or publishes an away-notice, an agent reads
one only when its house asks, and `list_away`'s description tells it to keep
what it read inside the conversation.

The Campground went the same day ("campground ok too"). The handler refuses
no label, so the rule that the room lives by — one row is one stay, no
amount, no total, no plate, no nationality — is in the tool description,
where the model reads it. `from_who` retention stays `TBD`, un-built, waiting
for the privacy note as before.

## The Houses room

At the foot of *Your house*, the `.actions` row holds `📦 Export everything`
(stewards), `Log out on this device`, and now **`+ MCP key`** on the right,
plain weight — the card is not for it (not `primary`), it is not an inner
addition (not `lesser`), it is not a dismissal (not `ghost`).

Pressing it opens an inline form under the row: a label field (*What runs
it*, placeholder *e.g. Claude on Ana's laptop*) and a `primary` *Create key*.
The answer replaces the form:

```
┌──────────────────────────────────────────────────┐
│ potok_9f3a2c…                                    │  ← wraps, break-all
│ …7b1e0dc21e                                      │
└──────────────────────────────────────────────────┘
https://…/api/mcp   [📋 Copy]      [📋 Copy the key]   ← lesser · primary

Copy the key now — the portal keeps only its fingerprint and
cannot show it again. It acts as your house in every room it
reaches. Remove it above, like a phone.
```

Not `.pin`: that class is 2.1 rem with letter-spacing for six digits, and 70
characters in it pan a 360 px phone sideways. `.mcp-key` is the same dashed
brass frame in a wrapping monospace block with `word-break: break-all`. Copy
the key is the one `primary` act, the address is the lesser line, built from
`location.origin`, so dev shows the dev address. The devices list refreshes
and the new row shows `🤖 <label>` with a tag *agent* where a phone shows 📱;
its Remove button is the one that exists. Nothing else in the room moved.

Slovenian entries, to be checked by a Slovenian-speaking house like the
others in `later.md`:

| Key | Slovenian |
| --- | --- |
| + MCP key | + MCP ključ |
| Create key | Ustvari ključ |
| Copy the key | Kopiraj ključ |
| What runs it | Kaj ga uporablja |
| e.g. Claude on Ana's laptop | npr. Claude na Aninem prenosniku |
| Copy the key now — the portal keeps only its fingerprint and cannot show it again. | Kopirajte ključ zdaj — portal hrani samo njegov prstni odtis in ga ne more več pokazati. |
| It acts as your house in every room it reaches. Remove it above, like a phone. | Deluje kot vaša hiša v vsaki sobi, ki jo doseže. Odstranite ga zgoraj, kot telefon. |
| agent | agent |

## Explicitly out of scope

- OAuth for MCP clients that demand it (claude.ai connectors). A static key
  in a header is the whole auth story.
- Resources, prompts, sampling, server-to-client notifications: tools only.
- A read-only key. One column and one check away; waits for a house that
  asks for an agent that may look but not touch.
- A mark on rows saying an agent wrote them, or per-person anything.
- A stdio shim in the repo.
- Raw REST for agent keys (the denylist alternative).

## Decisions

| Decision | Outcome (owner, 2026-09-09) |
| --- | --- |
| Build it at all | Built, as option A |
| The fence: `/api/mcp` only, or a full login with a route denylist | **`/api/mcp` only.** `TestMCPFence` pins that a key cannot reach `/api/away`, `/api/devices`, `/api/pair` or any other route directly. It swings both ways: a phone's session token is answered 403 at `/api/mcp`, or a steward could paste what its browser holds into an agent and reach `ownerOrSteward` on every house's rows — the no-stewardship decision would have a side door |
| Stewardship on a key | **None for now.** "Later key might carry stewardship" — a possibility left open, not a closed question |
| Watchtower on the surface | **On.** A key is the logged-in house; the provider is a third party the house chose. No digests, no feeds still. Said as the neighbour hears it: a neighbour's assistant may read your notice, as a neighbour's phone may — which is why the privacy note owes the village a line (`later.md`) |
| Campground on the surface | **On.** The rule rides in the tool description. `from_who` retention stays `TBD` |
| Contacts on the surface (2026-09-10, when the room was built) | **On, reads and writes.** Same line as the Watchtower: a key is the logged-in house, the provider a third party that house chose. The numbers are of people who never joined the portal, so the description carries the Watchtower's sentence about keeping what it read in the conversation |
| Project pictures through MCP | **Add one, no delete.** The existing handler and its cap |
| Deletes for agents | **None** |
| What an agent puts in `author` | **Nothing**, unless the person dictated the words |
| Expiry | **None**, like a phone |
| The button's name | **`+ MCP key`** — the owner's literal string |
| The `potok_` prefix | Yes |
| The audience | The owner's agent **and a villager's** — the doc and the README are written for both |
| Slovenian strings above | `TBD`, a Slovenian-speaking house |
| OAuth for claude.ai connectors | Out of scope; revisit if a house uses claude.ai and not Claude Code |

---

## Appendix: what the code does

### Transport

- `POST /api/mcp`, `requireHouse` in front like every route. A client without
  the header gets the plain 401 the app already speaks, which every MCP
  client renders as "authentication failed" — the right signal.
- JSON-RPC 2.0, one request per POST, answered as `application/json`. No
  SSE, no server-initiated stream: `GET /api/mcp` answers 405. No sessions:
  the server issues no `Mcp-Session-Id`, which the spec allows for a
  stateless server. A JSON array (a batch) is refused with `-32600`; the
  2025-06-18 revision dropped batching.
- Methods: `initialize` (echoes `2025-03-26` when the client says so, else
  answers `2025-06-18`; capabilities `{"tools":{}}`; `serverInfo`
  `{"name":"mokri-potok-portal"}`; `instructions` with the two clocks, the
  `author` rule and the away-notice rule), any `notifications/*` (202, empty
  body), `ping` (`{}`), `tools/list`, `tools/call`. Anything else is
  `-32601`; unparseable is `-32700`; an unknown tool is `-32602`.
- `tools/call` returns `{"content":[{"type":"text","text":<the API's JSON
  as it is>}],"isError":<status ≥ 400>}`. A 204 reads as `{"ok":true}`. The
  API's error string is the tool's error text, so an agent reads "ends_at is
  before starts_at" and not a stack.
- The body cap is **3 MiB**, not `readJSON`'s 64 KiB: a project picture rides
  in as base64, and 2 MB of photo is ~2.7 MiB of text. `readPhoto` still caps
  the decoded bytes at 2 MB.
- Tool descriptions are English: a model reads them, not a villager, so `t()`
  does not apply — the one place where that is true, and this sentence is why.

### Dispatch

```go
type mcpTool struct {
	Name, Desc   string
	Method, Path string         // "/api/events/{id}/signup"
	Schema       map[string]any // JSON schema of the arguments
	Image        bool           // body is base64 `data` + `content_type`, not JSON
}
```

Path placeholders are named after the argument that fills them (`{id}`,
`{project_id}`, `{subject}`); the dispatcher fills them, sends the rest as
the JSON body (GET tools take none — no list route reads the query string),
copies the `Authorization` header the MCP request arrived with, sets the
`viaMCP` context value on the sub-request, and runs it through `s.Handler()`
into a small in-memory `ResponseWriter`. The handler that answers is the one
the phone talks to.

`requireHouse` reads `d.agent`; if set and the request is neither
`POST /api/mcp` nor marked `viaMCP`, it answers 403 `"this key speaks MCP
only"`. `IsSteward` is `false` for an agent. Because the mark is a context
value set only by the dispatcher, no header from outside can forge it, and
`TestMCPFence` sends a forged header to prove it.

Why not call the handlers' internals directly: because the handlers *are* the
rules. `ownerOrSteward`, the 409 on a stale codex `rev`, the answer `yes` a
creator gets on its own event, the run-moved push to riders — all of it lives
in the handler, and a second path would have to repeat it and would drift.
The cost, paid knowingly: the tool table names the same JSON fields the
handlers read, and nothing in the code keeps them aligned. `TestMCPFlow`,
`TestMCPWatchtowerAndCamp` and `TestMCPProjectPhoto` walk the write fields
of the event, away, camp, project and picture tools with valid arguments;
the other write tools are pinned only to reach their route. A renamed field
there surfaces to the first agent as the handler's own 400, not to a test.

### Migration

`020_agent_devices.sql`: `ALTER TABLE devices ADD COLUMN agent INTEGER NOT
NULL DEFAULT 0;`. What an agent device inherits that means nothing, and why it
is left alone: `quiet_ok` (it has no push subscription and cannot reach
`PUT /api/me/device` anyway) and `last_seen` (useful: the room shows when the
key last acted).

### Routes

| Route | Guard | What |
| --- | --- | --- |
| `POST /api/devices/agent` | `requireHouse` | `{label}` → 201 `{id, token}`. Refused for a common place (`kind='common'` has no login and must not grow one) and for an agent device ("a key does not mint keys") |
| `GET /api/devices` | `requireHouse` | Each row carries `agent` |
| `DELETE /api/devices/{id}` | `requireHouse` | Unchanged; it is the revoke. Unreachable with a key |
| `POST /api/mcp` | `requireHouse` | The MCP endpoint |
| `GET /api/mcp` | — | 405 |
| every other route | `requireHouse` | 403 for an agent device unless the request came through the dispatcher |
| every `requireSteward` route | | Denies an agent device: `IsSteward` is false when `agent = 1` |

### Tests

`mcp_test.go`, in the style of `village_test.go`:

- `TestMCPFlow`: mint, `initialize` in three versions, a notification is a
  202, `ping`, `tools/list`, `whoami`, `create_event` → the phone sees the
  event with the house's `yes`; a refused write carries the handler's words; a
  204 reads as ok; the devices list shows one agent.
- `TestMCPFence`: the key gets 403 on `/api/me`, `/api/events`, `/api/away`,
  `/api/camp`, `DELETE /api/devices/{id}`, `/api/pair`, `POST
  /api/devices/agent`, `PUT /api/houses/{id}`, push, `DELETE /api/events`,
  `/api/export`; `GET /api/mcp` is 405; a steward's key is 403 on the steward
  routes and "not yours" on another house's project in-process; a forged
  mark header changes nothing. And the other way through the door: a phone's
  own session token — a steward's and an ordinary house's — is 403 at
  `POST /api/mcp`; an `id` of `..`, `.`, `1/2`, `1?x=1` or empty is a
  JSON-RPC `-32602` `bad id` (invalid params, not `isError`: the argument
  never formed a request, so no tool ran); and `whoami` reports the request's
  `is_steward`, so a steward's key reads 0 while that house's phone reads 1.
- `TestMCPSurface`: no tool deletes, none reaches devices, pairing, push,
  `/api/me/*`, export, a house write or a steward route; every tool lands on
  a registered handler; the count is pinned at 46; unknown tool, unknown
  method, a batch and garbage are refused.
- `TestMCPKeyLifecycle`: a common place cannot mint, a key cannot mint,
  Remove turns the key into a 401.
- `TestMCPWatchtowerAndCamp`: read the Watchtower, become the watcher, get
  "not yours" on another house's dates, read and write the away thread, and
  walk a camper's stay arrived → held → handed.
- `TestMCPProjectPhoto`: a small PNG lands, padded or not; over 2 MB is the
  handler's 413; a gif is its 415; the key cannot read picture bytes.
