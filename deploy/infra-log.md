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
- `deploy/controller/poll.yaml`: second poll entry for this repo, 60 s.
- `deploy/app/Caddyfile`: `handle /potok/*` → `potok-api:8788`.
- `task doco:sync` there to restart the controller with the new poll config.

### 3. First deploy
- Pushed `main`; CI built the image and rolled `BE_TAG`; doco-cd created the
  `mokri-potok` project. First boot printed the bootstrap code to the container
  log (`task vm:logs`); the owner founded the village with it.
