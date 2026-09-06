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
  decision recorded in the plan. A **common place** (`houses.kind = common`:
  event grounds, parking) shares the table because the map colours it, but it
  is land, not an account: no invite, no login, never offered where a house is
  meant (hand a task to, notifications, invite links). The UI keeps it in its
  own small block.
- **One origin.** The Go container serves the API *and* the built frontend
  (`static.go`, embedded at image build). The portal lives at its own domain,
  in `SITE.md` of the homestead repo. No CORS, no second host. GitHub Pages is
  the retiring copy, not the home.
- **No third party in the page.** Weather is fetched by the backend from ARSO
  and trimmed (`weather.go`, cached 30 min), never framed. ARSO's terms require
  naming the source, so the panel shows *Vir: ARSO*; the fetch sends a
  User-Agent with a contact URL. **The panel is a forecast for a town some
  kilometres away, not a measurement at the village** — the page says so, and
  it must never become a frost source for the homestead work. An iframe would hand
  the agency every villager's address on a logged-in page. Any future embed
  gets the same treatment or an argument written down here.
- **Nothing is public.** Every API route except `/api/healthz`, `/api/status`,
  `/api/bootstrap`, `/api/join` requires a bearer token. **No unauthenticated
  write surface, ever** — a camper self-check-in link from park4night was asked
  for on 2026-09-06 and refused for that reason. The one public image
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
- **An answer is not a headcount.** A sign-up is `yes`, `no` or `maybe`, and
  silence is a fourth thing. Only `yes` is counted; never fold `maybe` into it.
  Moving an event's time bumps `events.time_version`, which marks every earlier
  answer stale — a headcount for a day that no longer exists is worse than none.
- **Any house may edit an event — provisional.** Owner's decision 2026-09-06:
  every account today is a villager. `events.edited_by` records who, and the
  room shows it. When a reduced "viewer" role arrives for volunteers, this
  narrows to the creator and stewards.
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
- **A task is taken by any house, or handed to one by its creator.** Any
  house takes a free task and lets it go. The task's creator, the project's
  creator or a steward may assign a house (agreed in real life first) or clear
  the holder — the assigned house is told (owner's decision 2026-09-05, evening,
  reversing the afternoon's "never assigned": the reminder that will one day
  read `due_at` replaces the assigner nagging in person). No other house
  touches `assigned_to`. The project's creator hears when a task is taken. "3 of 5
  done" is a project's progress; never show a house's count of tasks.
- **The campground holds no amounts.** One row = one camper's stay: a house
  noticed it (arrived), a house has the money (held), it reached the box
  (handed). A tick on arrival ("I already have the money") lands the row in
  handed at once — to be revisited. A note is optional. No `amount` column, no sums, no plates
  in the label ("grey camper", "family from NL"). The cash box is the ledger.
  Who the money is handed *to* is `TBD` — the treasurer question in
  `docs/design-more-rooms.md`. Retention of `from_who` (a steward clears labels
  older than 12 months, by a button) is `TBD` and not built; a privacy note
  decides it. The `camp` push carries the house and the camper label, never
  the note.
- **Done is a state, never a deletion.** Finished projects and closed tasks
  stay readable with their closing notes. Nothing archives itself.
- **Exit is designed.** `GET /api/export` (steward) dumps everything as JSON;
  `VACUUM INTO` backups land nightly in `${DATA_DIR}/backups/`. Keep both working.
- **No secrets in this repo, and none needed.** The first steward code is
  generated on first boot and printed to the container log. The VAPID key pair
  for web push is generated on first use and kept in the `settings` table.
- **Push is opt-in per phone, filtered per house, mutable village-wide.** A
  phone subscribes after the villager taps Allow and records the language it
  subscribed in; the house switches kinds off in `notify_off` (empty = all
  nine kinds on); a steward mutes a kind for everyone in `notify_off_global`,
  which records who and when, shown to every house. Both lists are checked on
  every send — **except an alarm, which rings through both mutes and quiet
  hours.** The fire bell is not a preference. The author's
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
- **The way back in.** When nobody holds a session — a steward's phone wiped,
  the origin changed — `docker exec <container> /server code "<house>"` on the
  VM rotates that house's invite and prints the link (`cli.go`, `task vm:code`).
  SSH to the machine is the credential; there is no in-app password to reset
  and no email to send. Keep it that way.
- **Codes.** A steward's invite is 10 characters and multi-use for 14 days — it
  travels through WhatsApp and lets a whole house in. A pairing code is 6
  digits, single use, 15 minutes, and only ever adds one more phone to a house
  that is already inside. The small keyspace is paid for by a per-IP cap on
  `/api/join` and by burning every live pairing code after five wrong guesses.
  Do not widen either without redoing that arithmetic.
- **One date control.** Every date or date-time in the app is `DatePicker.tsx`
  (parchment button + month grid in the calendar's own classes). Never a native
  `type=date` / `datetime-local` again — they look foreign and cannot prefill a
  month without a day. Values stay plain strings: `YYYY-MM-DD` or
  `YYYY-MM-DDTHH:MM`.
- **Buttons come in three weights.** `primary` for the one action a card or
  form is for, plain for the rest, `lesser` for inner additions (add a task, add
  a note) — a real button one size down, never a ghost link. `ghost` is for
  dismiss and delete only.
- **Slovenian first.** Every user-facing string goes through `t()` in
  `frontend/src/i18n.tsx` with a Slovenian entry. English is the fallback key.
- **The bottom bar fits five.** Village plus four rooms — Hall, Projects,
  Market, Shed (owner's pick 2026-09-05). A new room either replaces one, merges
  into one, or lives off the bar like Watchtower, Campground and Houses do —
  tiles on the Home map, two taps from anywhere. Projects has a second door: the
  📋 chip on a calendar event. Rooms carry a `short` label for
  the bar because a phone gives each item about 60 px. Rationale and the
  options rejected: `docs/design-more-rooms.md`.
- **Old notification links must keep working.** Payload URLs live in the
  database of no one — they are already on people's phones. `/bell` survives as
  a route alias after the merge; a post links to `#/tavern?at=board`.

## Layout

```
backend/            Go: main.go, internal/{config,store,httpapi}; migrations embedded
frontend/           Vite + React; src/rooms/* one file per room (Projects.tsx holds list + page). Hall.tsx = Calendar.tsx + Board.tsx
                    stacked, because the tavern is one door; src/map/VillageMap.tsx
frontend/public/    manifest.webmanifest, sw.js (push only, no caching), icons, backdrop.jpg (aerial photo behind the gate)
                    src/push.ts, src/Install.tsx, src/AddPhone.tsx, src/photo.ts (auth'd photo fetch + browser-side shrink)
backend/internal/httpapi/shed.go   tool photos as BLOBs (≤2 MB, served with auth), wishlist; remind.go = return nudges
                    events.go = RSVP + comments; weather.go = ARSO, server-side; static.go = the embedded frontend
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
