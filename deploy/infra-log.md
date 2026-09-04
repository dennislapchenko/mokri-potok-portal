# Infra log — what was done by hand, in order

> Chronological on purpose — the one file here allowed to be a changelog.
> Companion to `README.md` in this directory (target state).

## Facts (current)

- **VM:** the gaias-choice VM (Hetzner, Helsinki). SSH
  `ssh -p 13337 -i ~/.ssh/gaia root@gaias-choice.gardenofatlantis.com`.
  Owned by the gaias-choice repo; this stack is a tenant.
- **API URL:** `https://gaias-choice.gardenofatlantis.com/potok/api` (Caddy
  route in gaias-choice, prefix stripped).
- **Frontend:** `https://dennislapchenko.github.io/mokri-potok-portal/` (GitHub Pages, workflow build).
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

### 4. Known issue — phone sessions
- A house that opens its invite link from WhatsApp lands in WhatsApp's in-app
  browser. The token is stored in *that* browser's storage. Opening the site
  later in Safari/Chrome finds no token and the gate shows. Options and the
  recommendation are in the owner's homestead repo, `NOW.md` 🏰 block.
