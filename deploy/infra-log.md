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
- `deploy/app/Caddyfile`: `handle /potok/*` → `potok-api:8788`. **Not yet
  applied** — the assistant's edit to that file was blocked by its permission
  classifier; the owner adds the block (text in `deploy/README.md`) and pushes,
  doco-cd force-recreates Caddy with the new file.

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
- Container healthy. The bootstrap code is in `task vm:logs`; the village is
  not founded yet.
