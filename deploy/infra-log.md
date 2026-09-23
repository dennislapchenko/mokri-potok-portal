# Infra log — what was done by hand, in order

> Chronological on purpose — the one file here allowed to be a changelog.
> Companion to `README.md` in this directory (target state).

## Facts (current)

- **VM:** the gaias-choice VM (Hetzner, Helsinki). SSH
  `ssh -p 13337 -i ~/.ssh/gaia root@gaias-choice.gardenofatlantis.com`.
  Owned by the gaias-choice repo; this stack is a tenant.
- **URL:** `https://vas.mokri-potok.si` — page and `/api` from the one
  container, the whole host proxied by the gaias-choice Caddy.
- **Data:** `/srv/mokri-potok/data/potok.db` + `backups/` on the VM.
- **Image:** `ghcr.io/dennislapchenko/mokri-potok-portal-be`, tag rolled by CI into `.doco-cd.yml`.

## Timeline

### 1. Repo and Pages
- Created the public repo, enabled Pages with the GitHub Actions build type.

### 2. VM wiring (in the gaias-choice repo)
- `deploy/controller/poll.yaml`: second poll entry for this repo, 60 s. Committed
  and synced with `task doco:sync` there. **`up -d` alone does not reload the
  poll config** (the daemon reads the file at start) — a
  `docker compose restart doco-cd` in `/opt/doco-cd` was needed.
- `deploy/app/Caddyfile`: `handle /potok/*` → `potok-api:8788`. Applied by the
  owner (gaias-choice commit "feat: add potok portal temp routing"); doco-cd
  force-recreated Caddy. `GET /potok/api/status` answers 200 from the internet.

### 3. First deploy
- CI built the image (first push failed with `unknown blob` on the provenance
  attestation; `provenance: false` fixed it) and rolled `BE_TAG`. The GHCR
  package came out public without a manual flip.
- doco-cd created the `mokri-potok` project. **First boot crash-looped:** the
  image runs as `nonroot` (uid 65532) and docker had created the bind-mount dir
  `/srv/mokri-potok/data` as root 755, so SQLite could not open its file
  (`unable to open database file: out of memory (14)`). Fixed on the host with
  `chown -R 65532:65532 /srv/mokri-potok/data` + `docker restart`. A fresh VM
  needs that chown before the first deploy.
- Container healthy. The owner founded the village from the Pages URL; three
  houses exist as of 2026-09-04.

### 4. Own domain, own frontend — 2026-09-06
- The village created an A record **vas.mokri-potok.si → this VM**.
- `backend/Dockerfile` now builds the frontend and embeds it in the Go binary,
  so one container answers the API and the page. Build context moved to the
  repo root; the CI workflow rebuilds on `frontend/**` too.
- Caddy routing for the new hostname lives in the **gaias-choice** repo
  (`deploy/app/Caddyfile`, `POTOK_DOMAIN`), which owns the edge. The old
  `/potok/*` path stays for the Pages frontend until it retires.
- **Push subscriptions are bound to an origin.** Everyone who allowed
  notifications on the Pages origin must allow them again on the new domain;
  the install banner asks. Nothing warns them automatically.
- `PUSH_SUBJECT` and `WEATHER_LOCATION` are the two new env knobs.
  **2026-09-23:** the binary lost its village defaults on the way to
  `porta-pagi`. `VILLAGE_NAME`, `PUBLIC_URL` and `PUSH_SUBJECT` are required
  and set in `deploy/app/compose.yaml`; `WEATHER_LOCATION` too, since unset now
  means no panel. `POTOK_BOOTSTRAP_CODE` is `PAGI_BOOTSTRAP_CODE` (unused here:
  the code was generated on first boot).
  Same day, after the roll of `79e71b4`: `task vm:map` seeded the `parcels`
  (GURS, snapshot 2026-08-15) and `water` rows of `map_files` from
  `deploy/map/`. The map read from the database from that minute; the two
  minutes between the roll and the seed showed villagers "no map yet".
- **Recovery:** `docker exec mokri-potok-potok-api-1 /server code "<house>"`
  prints a fresh invite link for that house. Needed after the move, because a
  session lives in one browser on one origin and the origin changed.
- The GitHub Pages copy was replaced by a signpost page on the same day, so
  there is no second half-broken app answering the same database.
- **What one container costs:** a bad migration now takes the page down with
  the API, and rolling the frontend back rolls the backend with it. The trade
  bought one origin, no CORS and no version skew between page and API.

### 5. Phone sessions and push
- Likely cause of lost sessions (unconfirmed on a phone): an invite link opened
  from WhatsApp lands in WhatsApp's in-app browser and the token stays there.
- Shipped: web manifest + install banner (Android prompt, iOS Share → Add to
  Home Screen, in-app-browser warning), `navigator.storage.persist()`, service
  worker, VAPID web push with per-house kind toggles. The VAPID pair is
  generated on first `GET /api/push/key` and stored in the `settings` table —
  it is inside the nightly SQLite backup. Losing it would silently orphan
  every subscription; phones would need to re-enable.
- `PUSH_SUBJECT` (VAPID subject) defaults to `https://vas.mokri-potok.si/`;
  set it in `deploy/app/compose.yaml` if the domain moves.
- `TZ=Europe/Ljubljana` is set on the service and the binary embeds
  `time/tzdata` (distroless carries no zoneinfo). Without it a notification
  would say "tomorrow" for tonight. Change both together if the village moves.

### 6. Pages taken down — 2026-09-08
- Everyone had moved to `vas.mokri-potok.si`. The Pages site was disabled
  (`gh api -X DELETE repos/dennislapchenko/mokri-potok-portal/pages`), its
  workflow deleted, and the Pages origin dropped from `CORS_ORIGINS`; the
  compose file no longer passes that variable, and since the same day the
  backend reads none. The old `/potok/*` route
  left the gaias-choice Caddyfile: that repo's `deploy/infra-log.md`, same date.

### 7. The split — 2026-09-23
- The code moved to `github.com/dennislapchenko/porta-pagi` with its whole
  history (a merge of unrelated histories, no rewrite). This repo keeps the
  village's deploy. `compose.yaml` was pointed at `ghcr.io/dennislapchenko/
  porta-pagi` and `BE_TAG` at `sha-1fbbe4d…`, porta-pagi's first build — the
  same source as the last `mokri-potok-portal-be` build, so that roll should
  change no table. CI here is gone; `task roll -- sha-<commit>` is the release.
- **Rollback of that first roll is a hand edit**, not `task roll`: the old
  tags live under the old image name, so set `image:` back to
  `ghcr.io/dennislapchenko/mokri-potok-portal-be` in `compose.yaml` and
  `BE_TAG` to `sha-7a6f2c3…`.
- The porta-pagi package was flipped to public by the owner in the package's
  settings (there is no API for it), the anonymous pull answered 200, the cut
  was pushed and doco-cd rolled the village to
  `ghcr.io/dennislapchenko/porta-pagi:sha-1fbbe4d…` at 20:13: healthy, no
  migration in the log, `/api/status` unchanged. The VM pulls anonymously, so
  the package must stay public; the source repo stays private for now.
