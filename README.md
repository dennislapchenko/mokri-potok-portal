# mokri-potok-portal

Closed web portal for the houses of the Mokri Potok village: a cadastral map of
the village as the home screen, and six rooms — Tavern (pinned notices, the
month calendar with work-bee sign-ups, and the message board), Market (needs,
give-aways, shop runs), Watchtower (who is away), Tool shed (what the village
lends), Projects (long jobs split into takable tasks) and Campground (who
collected from which camper). Hall, Projects, Market and Tool shed sit on the phone's
bottom bar, the rest are tiles on the map. One shared login
per house, handed out as an invite link in WhatsApp; a house adds its own
further phones with a six-digit pairing code. Installs to the home screen and
sends notifications.

- **Frontend:** Vite + React 18 + TypeScript, hash-routed SPA, Slovenian + English,
  installable (web manifest + service worker for push). Built **into the backend
  image** (`backend/Dockerfile`, build context is the repo root) and served by the
  Go binary, so the portal is one origin on its own domain. GitHub Pages now
  publishes only a signpost to the new address
  (`.github/workflows/deploy-pages.yml`).
- **Backend:** Go, stdlib HTTP, SQLite (pure Go), VAPID web push, one binary, nightly backups in-process.
  Image built by CI to GHCR; deployed by the doco-cd controller on the gaias-choice
  VM (`.doco-cd.yml`, `deploy/`).
- **Map data:** `frontend/public/data/parcels.geojson` — public cadastre (GURS), EPSG:3794.
  House ↔ parcel assignment is app data, never in git.

```sh
task check      # vet + test backend, typecheck + build frontend
task be:run     # backend on :8788 (bootstrap code printed in the log)
task fe:dev     # frontend on :5173 against the local backend
task vm:logs    # backend logs on the VM
task vm:code -- "Solnce"   # fresh invite link for a house, when nobody is logged in
```

Design plan and decisions live in the owner's homestead repo
(`70-collective/village-app/plan.md`). Read `CLAUDE.md` before changing anything.
