# deploy/

Target state of the backend stack on the shared VM. What was actually done by
hand, in order, is `infra-log.md`.

- `app/compose.yaml` — one service, `potok-api`, on the GHCR image pinned by
  `BE_TAG` in `/.doco-cd.yml`. Joins `gaias-choice_default` as an external
  network so that repo's Caddy can `reverse_proxy potok-api:8788`.
- Data: host bind mount `/srv/mokri-potok/data` (SQLite + `backups/`).
- Edge: none here. `https://gaias-choice.gardenofatlantis.com/potok/*` is a
  `handle` block in the gaias-choice repo's `deploy/app/Caddyfile`, placed
  before its catch-all `handle`; the prefix is stripped so the backend sees
  `/api/...`:

  ```caddyfile
  handle /potok/* {
  	uri strip_prefix /potok
  	reverse_proxy potok-api:8788
  }
  ```
- Host prerequisite: `/srv/mokri-potok/data` owned by uid 65532 (the image
  runs as distroless `nonroot`).
- Controller: the doco-cd daemon in `/opt/doco-cd` on the VM belongs to the
  gaias-choice repo (`deploy/controller/`). This repo is its second poll entry.

Release = push to `main` touching `backend/**`: CI builds, pushes, rolls
`BE_TAG`; doco-cd reconciles within its poll interval.
