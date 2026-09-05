# CLAUDE.md

1. Don't assume. Don't hide confusion. Surface tradeoffs.
2. Minimum code that solves the problem. Nothing speculative.
3. Touch only what you must. Clean up only your own mess.
4. Define success criteria. Loop until verified (`task check` is the floor).

## What this is

The village portal for a small Slovenian collective — see `README.md` for the
stack and `.claude/context/village.md` for who uses it and why the rooms are
shaped the way they are. It is a **social tool for a handful of houses**, not
a product: every feature must beat "scroll up in the WhatsApp group" or it
does not ship.

## Invariants

- **A house is the account.** Everyone in a house shares one identity; a device
  row per phone tells them apart. Never add per-person accounts without a
  decision recorded in the plan.
- **Nothing is public.** Every API route except `/api/healthz`, `/api/status`,
  `/api/bootstrap`, `/api/join` requires a bearer token. The one public image
  is `backdrop.jpg` behind the gate — the owner chose it knowing the repo is
  public; provenance `TBD`. Away-notices are
  burglary information: they never leave the logged-in app (no digests, no feeds).
- **No tallies of favours.** "Taken by", "claimed by", "watched by" are
  acknowledgments. No counts, points, leaderboards, streaks. Ever.
- **Cadastre is a view, not a source.** `parcels.geojson` is public GURS data;
  parcel numbers show only for assigned parcels (and to stewards in assign
  mode). The map carries its snapshot date and "boundaries, not fences" line.
  House ↔ parcel assignment lives in the DB only. Licence attribution for the
  GURS data: `TBD`, required before the portal moves to the collective's domain.
- **No tally of favours.** Work-bee sign-ups are a headcount for one day and
  die with the event. Tool loans record who holds a tool now, never how many
  times a house borrowed. Do not add a total anywhere. The **return reminder**
  (`remind.go`) nudges the holder after one day and then with doubling gaps
  (day 1, 3, 7, 15), read off two timestamps — never a counter, never the
  owner, never in quiet hours. A cap "after N reminders" would need a count;
  do not add one.
- **Photos stay in the database.** A tool photo is shrunk in the browser and
  stored as a BLOB, so the SQLite backup is the whole village. The list and the
  export never carry the bytes; `GET /api/tools/{id}/photo` needs a token.
  The export hardcodes the `tools` column list — a new `tools` column must be
  added there too, or it silently drops out of the exit path.
- **Wishlist names are not votes.** Never sort, badge or count by how many
  houses want a thing. A wish ends when the wisher marks it arrived.
- **Exit is designed.** `GET /api/export` (steward) dumps everything as JSON;
  `VACUUM INTO` backups land nightly in `${DATA_DIR}/backups/`. Keep both working.
- **No secrets in this repo, and none needed.** The first steward code is
  generated on first boot and printed to the container log. The VAPID key pair
  for web push is generated on first use and kept in the `settings` table.
- **Push is opt-in per phone, filtered per house, mutable village-wide.** A
  phone subscribes after the villager taps Allow and records the language it
  subscribed in; the house switches kinds off in `notify_off` (empty = all
  seven kinds on); a steward mutes a kind for everyone in `notify_off_global`.
  Both lists are checked on every send. The author's
  house never receives its own event. Push carries a title, a one-line snippet
  and a route, in human words — `banner_test.go` pins every string, so read it
  before changing copy.
- **What a lock screen may say about an empty house.** Anyone holding a phone
  can read a notification. The line: a **multi-day absence is anonymous** — an
  away push names no house, no dates, no notes, only "new notice, open the
  Watchtower". A **scheduled short trip is named** — a shop run says who drives
  and when, because the house is coming back the same day and the whole point
  is to answer it. Classify a new kind against that line; do not guess.
  The one other exception is the optional author name on a tavern post, which
  the poster typed themselves.
- **Quiet hours: 21:00–07:00 only an alarm rings.** Everything else waits in
  the app rather than buzzing a neighbour at night. `s.now()` decides, so tests
  can move the clock.
- **Codes.** A steward's invite is 10 characters and multi-use for 14 days — it
  travels through WhatsApp and lets a whole house in. A pairing code is 6
  digits, single use, 15 minutes, and only ever adds one more phone to a house
  that is already inside. The small keyspace is paid for by a per-IP cap on
  `/api/join` and by burning every live pairing code after five wrong guesses.
  Do not widen either without redoing that arithmetic.
- **Slovenian first.** Every user-facing string goes through `t()` in
  `frontend/src/i18n.tsx` with a Slovenian entry. English is the fallback key.
- **The bottom bar fits five.** Village plus four rooms. A new room either
  replaces one, merges into one, or lives off the bar like Houses does (reached
  by tapping your own house name in the top bar). Rooms carry a `short` label
  for the bar because a phone gives each item about 60 px.
- **Old notification links must keep working.** Payload URLs live in the
  database of no one — they are already on people's phones. `/bell` survives as
  a route alias after the merge; a post links to `#/tavern?at=board`.

## Layout

```
backend/            Go: main.go, internal/{config,store,httpapi}; migrations embedded
frontend/           Vite + React; src/rooms/* one file per room. Hall.tsx = Calendar.tsx + Board.tsx
                    stacked, because the tavern is one door; src/map/VillageMap.tsx
frontend/public/    manifest.webmanifest, sw.js (push only, no caching), icons, backdrop.jpg (aerial photo behind the gate)
                    src/push.ts, src/Install.tsx, src/AddPhone.tsx, src/photo.ts (auth'd photo fetch + browser-side shrink)
backend/internal/httpapi/shed.go   tool photos as BLOBs (≤2 MB, served with auth), wishlist; remind.go = return nudges
docs/               design docs the owner and the assistant decide on together (navigation growth, Projects, Campground)
                    icon.svg is hand-drawn paths, full-bleed, content inside the central 80 % safe circle
                    (an emoji glyph renders off-centre and monochrome — do not go back to one)
frontend/public/data/  parcels.geojson (cadastre), channels.json (modelled water, dashed)
deploy/app/         compose for the VM stack; deploy/infra-log.md = what was done by hand
.doco-cd.yml        deploy config the VM's doco-cd polls; BE_TAG rolled by CI
.github/workflows/  build-backend (GHCR + roll tag), deploy-pages
```

## Docs stay current, in the same change

- **One fact, one home.** Architecture and invariants: this file. Stack and
  commands: `README.md`. Manual VM steps, in order: `deploy/infra-log.md`
  (the one place allowed to be a changelog). Village context: `.claude/context/village.md`.
- A change to the API shape, a table, the deploy path or a workflow updates the
  doc that owns it **in the same commit**. Docs describe current state — when
  something becomes history, delete it rather than framing it as a change.
- A new room or a new table gets one line in `village.md` saying which
  villager job it serves. If that line cannot be written, the feature is not needed.
- `.claude/launch.json` starts the dev frontend; `task be:run` the backend.

## Working rules

- Run `task check` before every commit. CI runs the same.
- Migrations are append-only files in `backend/internal/store/migrations/`;
  never edit an applied one.
- Prefer a few lines of code over a dependency. Backend has two (sqlite,
  webpush-go for VAPID + payload encryption); frontend has three runtime deps.
  Adding one is a decision to write down here.
- Commits: lowercase, succinct, say what changed and why. No AI trailers, no
  backticks in subjects (they break the Telegram deploy ping).
- Push to `main` deploys: frontend to Pages within ~1 min, backend image via
  GHCR then the VM's doco-cd within ~2 min of the roll commit. Verify with
  `task vm:logs` and the Pages URL, not by assumption.
- The VM is shared with gaias-choice: Caddy and the controller belong to that
  repo. A portal change that needs a new route or a new poll entry is a change
  **there** — see `deploy/infra-log.md`.
