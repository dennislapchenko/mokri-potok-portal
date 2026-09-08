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

- **A house is the account, with or without land.** Everyone in a house shares
  one identity; a device row per phone tells them apart. Never add per-person
  accounts without a decision recorded in the plan. **A member who lives here
  without land — renting a hut, using a vacant house part of the year — is an
  ordinary house whose parcel list is empty** (owner's decision 2026-09-06,
  option F in `docs/design-membership.md`). **No third kind of row**: a
  `kind='household'` would also make `ORDER BY kind, name` put them last in
  every list with nobody deciding it. What replaces the land on such a row is
  a mark on the map — `house_homes`, next bullet. **No sentence about where a
  house lives**: `houses.about` was dropped on 2026-09-08 (owner's decision),
  the mark is the answer. **The Houses room shows
  no parcel count**, in either block: a parcel list is a list, a count beside a
  name is a tally.
- **Marking a parcel somebody else holds says "we live here", never "this is
  ours"** (owner's decision 2026-09-07). In assign mode a tap on a free parcel
  assigns it as before; a tap on a parcel another house holds writes
  `house_homes` instead, so the crest joins that parcel on the map beside the
  holder's, and the Houses room reads *"Živi na zemlji hiše 🏠 …"* where a
  landholder has parcel numbers. It is never folded into `parcels`: a house
  that rents a hut holds no land and no list may say it does. The cost, paid
  knowingly: **a parcel no longer changes hands by assigning it to the new
  house** — clear it from the old one first.
  **The house itself marks where it lives** — `PUT /api/houses/{id}` takes
  `homes` from that house (a steward may write it too, because a steward keeps
  the map for houses that will not open a picker).
  A mark grants nothing: `homes` is not `house_parcels`, so a house cannot put
  itself on the map as a landholder, and a parcel it already holds is dropped
  rather than printed back at it. Marking a parcel nobody holds is allowed —
  living somewhere is a fact whether or not the app knows the owner yet.
- **A stay ends by deleting the house** (owner's decision 2026-09-06), and only
  a steward can. **This is the one deletion the app allows, and the deliberate
  exception to "done is a state, never a deletion" below.** Deleting cascades
  through everything that house wrote — posts, its replies inside other houses'
  threads, events, sign-ups, needs, give-aways, away-notices, its tools, wishes,
  projects, tasks, camp rows — so **the confirm dialog names them, and says the
  honest thing about undo**: last night's `VACUUM INTO` backup can bring the
  house back, at the price of rolling the whole village back to it, so the
  dialog tells the steward to export first. Never make that button quieter, and
  never let it claim the loss is absolute — it is a day, not forever. Pictures
  the house put on projects stay with the project and lose only their house
  (owner's decision 2026-09-08). Going home
  for the season is not leaving: that is an away-notice, same as any house that
  winters elsewhere. The softer variant (`houses.left_at`) is designed and not
  built — `docs/design-membership.md` § Off-season, and ending a stay.
  A **common place** (`houses.kind = common`:
  event grounds, parking) shares the table because the map colours it, but it
  is land, not an account: no invite, no login, never offered where a house is
  meant (hand a task to, notifications, invite links). The UI keeps it in its
  own small block.
- **One origin.** The Go container serves the API *and* the built frontend
  (`static.go`, embedded at image build). The portal lives at its own domain,
  in `SITE.md` of the homestead repo. No second host and no CORS: in dev the
  Vite server proxies `/api` to the backend (`vite.config.ts`), so the page is
  same-origin there too.
- **No third party in the page.** Weather is fetched by the backend from ARSO
  and trimmed (`weather.go`, cached 30 min), never framed. ARSO's terms require
  naming the source, so the panel shows *Vir: ARSO*; the fetch sends a
  User-Agent with a contact URL. **The panel is a forecast for a town some
  kilometres away, not a measurement at the village** — the page says so, and
  it must never become a frost source for the homestead work. An iframe would hand
  the agency every villager's address on a logged-in page. Any future embed
  gets the same treatment or an argument written down here. **The fonts are
  served from this origin** (`frontend/public/fonts/`, SIL OFL, licences beside
  the files). **The page carries a CSP** (`static.go`, on `index.html` only):
  everything `'self'`, inline styles allowed because React paints house colours
  into style attributes, `blob:` images for photos and the export. The Go
  server sets it, not Caddy, so it ships and is tested with the frontend it
  describes; the gaias-choice Caddyfile sets none for this host on purpose.
  **A house colour is `#rrggbb` and nothing else** — CSS `background` also
  takes `url(...)`, so an unchecked colour is a beacon fired from every
  villager's browser; the backend answers anything else with a 400.
- **Nothing is public.** Every API route except `/api/healthz`, `/api/status`,
  `/api/bootstrap`, `/api/join` requires a bearer token. **No unauthenticated
  write surface, ever** — a camper self-check-in link from park4night was asked
  for on 2026-09-06 and refused for that reason. The one public image
  is `backdrop.jpg` behind the gate — the owner chose it knowing the repo is
  public; provenance `TBD`. Away-notices are
  burglary information: they never leave the logged-in app (no digests, no feeds).
- **No tallies of favours.** "Taken by", "claimed by", "watched by" are
  acknowledgments. No counts, points, leaderboards, streaks. Ever.
- **Water on the map is one file and one component.** `map/Water.tsx` draws
  `public/data/water.json` — the watercourses chained out of an epsilon
  priority-flood + D8 model (the method of the homestead repo's
  `10-site/terrain-data/water.py`) run on ARSO DMR 1 m over **the whole village,
  E485000-488000 N44800-47600** — the box the map's own reset view opens on, so
  a course no longer stops in the middle of the page. Drawn: a catchment of
  **2 ha** or more and at least **250 m** long. That floor is set from below by
  the collective's own stream, which peaks at 2.36 ha — raise it and the plot's
  east boundary loses its water. **Named: 100 ha and above** (`stream_m2`), which
  is three courses and is the whole readability budget; the other 29 are drawn
  thin and **left unnamed on purpose**, because 1 m bare earth says how much
  land drains into a channel and never whether it runs. So there is **no
  *grapa* on the map any more** — the old rule named the plot's east course
  *potok* and the west one *grapa*, and at village scale the west course
  (0.34 ha, an eroded road) is below the floor and is not drawn at all.
  **The village is a closed karst basin** (outlet 525.8 m, floor 451.5 m, per
  `rimclose.py` in the homestead repo), so the fill is **capped at 2 m** and a
  real sink stays an outlet. Do not lift that cap: an uncapped priority-flood
  fills the basin to its rim, drowns every channel in one flat, and returns a
  confident hairball. **Do not put names on the lines.** OSM and the 2013
  survey disagree about this water — OSM calls the valley trunk *Reka* and puts
  *Mokri potok* 1.6 km west, `10-site/geodetic-survey-2013.md` reads *Mokri
  potok* on the trunk itself — and the owner has not picked. To take the water
  off the map, delete those two files and the `<Water/>` line in
  `VillageMap.tsx` — nothing else knows about it.
- **Cadastre is a view, not a source.** `parcels.geojson` is public GURS data;
  parcel numbers show only for assigned parcels (and to stewards in assign
  mode). The caption under the map is **one line**: the snapshot date and GURS.
  The "boundaries, not fences" sentence and the water-model disclaimer were cut
  on 2026-09-07 (owner's decision) — a caption nobody finishes reading protects
  nobody. The attribution itself stays, and an outline nobody holds is drawn at
  half stroke so the village reads out of the cadastre mesh.
  House ↔ parcel assignment lives in the DB only. Licence attribution for the
  GURS data: `TBD`, required before the portal moves to the collective's domain.
- **One thread implementation.** Comments live in `comments`, keyed by
  `(subject, subject_id)`, and are rendered by `Thread.tsx` everywhere: an
  event, a wish. One reply level. A new room that wants comments adds a subject,
  never a table. The push kind stays the room's own so no opt-out changes
  meaning. The name field starts filled from the device label this phone joined
  under, and stays optional.
- **Wish options are findings, not votes.** Anyone may add "this model, this
  price, this link" to a wish. Never count, rank or mark a winner.
- **A past event leaves the list, never the calendar.** The list under the month
  grid holds **that month's** events that are not over — not everything ahead
  forever — so it cannot grow past a month's worth, and ‹ › pages it with the
  grid. What has passed stays in its day cell, one click away. `isOver` in
  `rooms/shared.tsx` decides: an event is over when its **end** has passed, and
  an event with no end lasts until the end of its day, because dropping a work
  party a minute after it starts hides it from whoever is running late. The Home
  badge and the tavern peek use the same function — never re-derive it with a
  date comparison, which counts this morning's finished event as ahead.
- **An answer is not a headcount.** A sign-up is `yes`, `no` or `maybe`, and
  silence is a fourth thing. **The house that creates an event answers `yes` for
  itself in the same request** — whoever calls a work party is at it, and a
  headcount of nobody beside an event somebody called reads as a failure that
  did not happen. It is an ordinary answer: the caller can change it or take it
  back like any house. Only `yes` is counted; never fold `maybe` into it.
  It carries **no note** — the sign-up note was removed on 2026-09-06 and the
  old ones moved into the event's thread (`013_quiet_and_notes.sql`), because a
  line only the signer can edit and nobody can answer is a worse comment. A
  POST without a valid state is a 400, never a yes.
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
- **Photos stay in the database.** A tool photo, or a picture on a project, is
  shrunk in the browser and stored as a BLOB (`photos.go` reads and serves
  both), so the SQLite backup is the whole village. Lists and the export never
  carry the bytes; `GET /api/tools/{id}/photo` and `GET /api/photos/{id}` need
  a token. The export hardcodes the `tools` and `project_photos` column lists —
  a new column on either must be added there too, or it silently drops out of
  the exit path. **Any house may put a picture on any project** — a project is
  the village's, like its events — and the full-size view says which house and
  when; the house that added it, the project's house or a steward takes it
  down. The input opens the chooser, not the camera: before-and-after pictures
  are already in the gallery.
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
  phone may register only an `https://` endpoint on a dotted hostname —
  the backend POSTs to whatever it is given, and what actually keeps it off a
  compose neighbour is TLS verification, which the hostname check just
  fails early. A
  phone subscribes after the villager taps Allow and records the language it
  subscribed in; the house switches kinds off in `notify_off` (empty = all
  nine kinds on); a steward mutes a kind for everyone in `notify_off_global`,
  which records who and when, shown to every house. Both lists are checked on
  every send. **Nothing is exempt**: the alarm kind was removed on 2026-09-06
  because a real emergency is a phone call, and a notification nobody is holding
  is worse than none. An event is `event` or `work`, and that is all. The author's
  house never receives its own event. **A run's riders hear when it moves**
  (owner's decision 2026-09-08): editing a run's place or time pushes to the
  houses whose open needs sit on it, named after the driver whoever edited,
  and to nobody else — not the driver, not the editor; a notes edit rings
  nobody. Push carries a title, a one-line snippet and a route, in human words
  — `banner_test.go` pins every creation banner and `TestMarketEdits` the
  run-change one, so read them before changing copy.
- **What a lock screen may say about an empty house.** Anyone holding a phone
  can read a notification. The line: a **multi-day absence is anonymous** — an
  away push names no house, no dates, no notes, only "new notice, open the
  Watchtower". A **scheduled short trip is named** — a shop run says who drives
  and when, because the house is coming back the same day and the whole point
  is to answer it. Classify a new kind against that line; do not guess.
  The one other exception is the optional author name on a tavern post, which
  the poster typed themselves.
- **Quiet hours: 21:00–07:00 nothing rings, unless a phone asked.** Everything
  waits in the app rather than buzzing a neighbour at night. `s.now()` decides,
  so tests can move the clock. The one way through is `devices.quiet_ok`, set by
  that phone alone (`PUT /api/me/device`): the flag is on the **device**, not the
  house, because the phone on a bedside table belongs to a person and the other
  phones of that house did not choose. `nightFilter()` narrows the recipients
  instead of dropping the send, and a subscription with no device row behind it
  stays silent — the default is quiet. **Tool return reminders are the
  exception to the exception**: `remind.go` still skips the whole night cycle,
  so the nudge lands in the morning. A nudge is not news.
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
  `YYYY-MM-DDTHH:MM`. The far end of a range takes `min` (the near end's value,
  same shape): days and hours before it are **dead cells, never a silent
  clamp** — a picker that moves the day you pressed is worse than one that shows
  the door is shut. Moving the near end past the far end drags the far end
  along, because only the field you touched may change under you. `badRange()`
  in the backend refuses a backwards pair whatever the client sends. The popup
  cancels its clicks' default action: every room puts it inside a `<label>`, and
  a label forwards a click to its first control — here the field button, which
  reopens the popup you just closed. It also anchors to the field's right edge
  when the left would hang it off a narrow screen; a popup past the edge makes
  the page pan sideways and taps get eaten as scrolls.
- **Two clocks live in the database, and they never compare directly.** A time a
  villager typed is local wall clock, `YYYY-MM-DDTHH:MM`. Anything
  `datetime('now')` wrote is UTC with a space and seconds — SQLite ignores the
  container's `TZ`, which only reaches Go's `time.Now()` (so quiet hours are
  local). `When` in `rooms/shared.tsx` tells the two apart by that space and
  appends the `Z`; a bare stamp reads as local and shows the village two hours
  early in summer. SQL comparing a wall-clock column to now needs `strftime` in
  that column's own shape — `starts_at >= datetime('now')` compares `'T'` with
  `' '` and quietly answers wrong.
- **The button frame is the house style, not only the button.** A weather
  forecast day wears `.lesser`'s border, fill and thin shadow (`.w-day`) and is
  not clickable: no hover, no press. Reuse the frame where a small box needs to
  belong; never reuse the affordance.
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
frontend/public/    manifest.webmanifest, sw.js (push only, no caching), icons, backdrop.jpg (aerial photo behind the gate), fonts/ (self-hosted, OFL)
                    icon.svg is hand-drawn paths, full-bleed, content inside the central 80 % safe circle
                    (an emoji glyph renders off-centre and monochrome — do not go back to one)
                    src/push.ts, src/Install.tsx, src/AddPhone.tsx, src/photo.ts (auth'd photo fetch + browser-side shrink)
backend/internal/httpapi/shed.go   tool photo routes, wishlist; photos.go = how a photo is read and served (≤2 MB, auth) + project pictures; remind.go = return nudges
                    threads.go = comments on any subject + wish options; weather.go = ARSO, server-side; static.go = the embedded frontend
docs/               design docs the owner and the assistant decide on together (navigation growth, Projects, Campground,
                    and `design-membership.md` — accounts for people who live here without land, options only, nothing built);
                    `later.md` = the one home for what is brainstormed, designed-and-set-aside, or still undecided
docs/diagrams/      hand-drawn SVG sketches belonging to those docs
frontend/public/data/  parcels.geojson (cadastre), water.json (the modelled watercourses, drawn by map/Water.tsx)
deploy/app/         compose for the VM stack; deploy/infra-log.md = what was done by hand
.doco-cd.yml        deploy config the VM's doco-cd polls; BE_TAG rolled by CI
.github/workflows/  build-backend (GHCR + roll tag)
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
- `.claude/launch.json` starts the dev frontend or the backend; `task be:run`
  is the same backend from a terminal. Checking the CSP against a real page:
  `README.md`.

## Memory

@.claude/memory/MEMORY.md

In-repo memory lives in `.claude/memory/`: one file per fact, `MEMORY.md` the
index, same frontmatter shape as the assistant's global memory. It holds how
the owner likes to work, project constraints not derivable from code or git,
and pointers. It never holds what a doc already records, and never anything
private: the repo is public (first working rule below). Update a file rather
than adding a duplicate, delete one that turns out wrong.

## The reviewer

`.claude/agents/reviewer.md` is a Fable subagent that answers one question
about a finished turn: what should have been done better, as a
psychotherapist, an engineer, a UX/UI specialist and a homesteading
practitioner at once. **Every turn that changed files invokes it before
ending**, with what was asked and what was done, and then acts on the
feedback or surfaces it to the owner. `.claude/hooks/review-gate.py`, wired as
a Stop hook in `.claude/settings.json`, blocks the turn once if that was
skipped. The reviewer edits nothing.

## Working rules

- **This repo is public.** The village name and the public GURS and ARSO
  data are the only specifics in it. Nothing that identifies a person or
  describes a household comes in here: no names of people, no phone numbers,
  no house-by-house facts, no quotes from the owner's private homestead notes
  about people or tenure. Pointing at a file in that repo by path is fine, its
  content about people is not. Abstract it or leave it out. This applies to
  docs, commits, memory and agent files alike.
- Work on `main`. One person owns this repo; a side branch and a pull request
  only put a gate between a verified change and the village. Commit to `main`
  and push (owner's rule 2026-09-07).
- Run `task check` before every commit. CI runs the same.
- Migrations are append-only files in `backend/internal/store/migrations/`;
  never edit an applied one.
- Prefer a few lines of code over a dependency. Backend has two (sqlite,
  webpush-go for VAPID + payload encryption); frontend has three runtime deps.
  Adding one is a decision to write down here.
- Commits: lowercase, succinct, say what changed and why. No AI trailers, no
  backticks in subjects (they break the Telegram deploy ping).
- Push to `main` deploys, frontend and backend together: the image carries
  both (`backend/Dockerfile` builds the frontend), CI pushes it to GHCR and
  rolls `BE_TAG`, and the VM's doco-cd reconciles within ~2 min of that roll
  commit. Verify with `task vm:logs`, not by assumption. Only `backend/**`,
  `frontend/**` and the workflow itself trigger it — a docs-only commit builds
  nothing.
- The VM is shared with gaias-choice: Caddy and the controller belong to that
  repo. A portal change that needs a new route or a new poll entry is a change
  **there** — see `deploy/infra-log.md`.
