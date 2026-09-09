# mokri-potok-portal

Closed web portal for the houses of the Mokri Potok village: a cadastral map of
the village as the home screen, and six rooms — Tavern (pinned notices, the
month calendar with work-bee sign-ups, and the message board), Market (needs,
give-aways, shop runs), Watchtower (who is away), Tool shed (what the village
lends), Projects (long jobs split into takable tasks) and Campground (who
collected from which camper) — plus the Codex, the village's founding text,
editable by every house, as the fourth chip at the head of Home. Hall, Projects, Market and Tool shed sit on the phone's
bottom bar, the rest are tiles on the map. One shared login
per house, handed out as an invite link in WhatsApp; a house adds its own
further phones with a six-digit pairing code. Installs to the home screen and
sends notifications.

- **Frontend:** Vite + React 18 + TypeScript, hash-routed SPA, Slovenian + English,
  installable (web manifest + service worker for push). The changelog at the
  foot of Home is the last 90 days of the git log, filtered and written to
  `public/changelog.json` by `scripts/changelog.mjs` before every build. Built **into the backend
  image** (`backend/Dockerfile`, build context is the repo root) and served by the
  Go binary, so the portal is one origin on its own domain.
- **Backend:** Go, stdlib HTTP, SQLite (pure Go), VAPID web push, one binary, nightly backups in-process.
  Image built by CI to GHCR; deployed by the doco-cd controller on the gaias-choice
  VM (`.doco-cd.yml`, `deploy/`).
- **Map data:** `frontend/public/data/parcels.geojson` — public cadastre (GURS), EPSG:3794 —
  and `water.json`, the watercourses a terrain model finds across the whole village.
  House ↔ parcel assignment is app data, never in git.

```sh
task check      # vet + test backend, typecheck + build frontend
task be:run     # backend on 127.0.0.1:8788 (bootstrap code printed in the log); BIND unset = all interfaces, as in the container
# CSP check against a real page: build, copy dist into the embed dir, run the
# backend, open :8788 — then git checkout the placeholder index.html again.
task fe:build && cp -R frontend/dist/. backend/internal/httpapi/web/
task fe:dev     # frontend on :5173 against the local backend
task vm:logs    # backend logs on the VM
task vm:code -- "<house>"  # fresh invite link for a house, when nobody is logged in
task vm:codex -- codex.json  # import the adopted codex once (shape in backend/internal/httpapi/codex.go)
```

## For agents (MCP)

The portal's API as MCP tools: an agent acts **as your house** — same rows,
same pushes, same rules as a phone. For the owner's agent and for any
villager who runs one.

Get a key: Houses room → Your house → `+ MCP key`. It is shown once; the
portal keeps only its fingerprint. Then:

```sh
claude mcp add --transport http potok https://<the portal's domain>/api/mcp \
  --header "Authorization: Bearer potok_…"
```

Other clients, `.mcp.json` shape:

```json
{"mcpServers":{"potok":{"type":"http","url":"https://<the portal's domain>/api/mcp",
  "headers":{"Authorization":"Bearer potok_…"}}}}
```

What the agent can do: read and write in every room — board, calendar and
answers, threads, shop runs and needs, give-aways, the Watchtower, the shed
and its wishlist, projects, tasks and a picture on a project, the campground,
the codex, the weather. What it cannot: delete anything, mint keys or touch
devices, edit the house itself, or do a steward's work. **The key opens
`/api/mcp` and nothing else** — `curl` with it gets a 403 — so the tool list
is the whole surface.

Two clocks: send local wall clock `YYYY-MM-DDTHH:MM` (a date alone is
`YYYY-MM-DD`); what you read as `created_at` is UTC with a space. Leave
`author` empty unless the person dictated the words — a post with no name
reads as the house, which is true. claude.ai's web connectors need OAuth and
will not connect. Claude Code takes the header as above; another client whose
config takes a header should work the same, but none was tried.

Revoke: Your devices → Remove on the 🤖 row. The key is your house's login in
plain text in a config file on a laptop — treat it like the invite link.
Dev: `http://127.0.0.1:8788/api/mcp` against `task be:run`.

Design plan and decisions live in the owner's homestead repo
(`70-collective/village-app/plan.md`). Read `CLAUDE.md` before changing anything.
