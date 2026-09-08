# deploy/

Target state of the backend stack on the shared VM. What was actually done by
hand, in order, is `infra-log.md`.

- `app/compose.yaml` — one service, `potok-api`, on the GHCR image pinned by
  `BE_TAG` in `/.doco-cd.yml`. Joins `gaias-choice_default` as an external
  network so that repo's Caddy can `reverse_proxy potok-api:8788`.
- Data: host bind mount `/srv/mokri-potok/data` (SQLite + `backups/`).
- Edge: none here. The gaias-choice repo's `deploy/app/Caddyfile` proxies the
  whole `vas.mokri-potok.si` host to `potok-api:8788`.
- Host prerequisite: `/srv/mokri-potok/data` owned by uid 65532 (the image
  runs as distroless `nonroot`).
- Controller: the doco-cd daemon in `/opt/doco-cd` on the VM belongs to the
  gaias-choice repo (`deploy/controller/`). This repo is its second poll entry.

Release = push to `main` touching `backend/**`: CI builds, pushes, rolls
`BE_TAG`; doco-cd reconciles within its poll interval.
