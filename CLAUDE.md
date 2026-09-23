# CLAUDE.md

1. Don't assume. Don't hide confusion. Surface tradeoffs.
2. Touch only what you must. Clean up only your own mess.
3. Verify on the VM (`task vm:logs`), never by assumption.

## What this is

The deploy repo of one village — the collective that built **Porta Pagi**
(`github.com/dennislapchenko/porta-pagi`, the product: code, invariants,
design docs). This repo holds what is this village's alone: its compose file
and env, the tag it runs, its map seed, its VM notes, and who it is
(`.claude/context/village.md`). Nothing here is code; a change to the portal
is a commit in `porta-pagi`, a CI build there, and a tag roll here.

## What lives where

```
deploy/app/compose.yaml   the one service, potok-api, on ghcr.io/dennislapchenko/porta-pagi pinned by BE_TAG;
                          the village's env: VILLAGE_NAME, LANGUAGES, PUBLIC_URL, PUSH_SUBJECT, WEATHER_*, TZ
.doco-cd.yml              what the VM's doco-cd polls; BE_TAG is the image the village runs — rolled by `task roll`
deploy/map/               parcels.geojson (GURS cadastre, snapshot 2026-08-15; the GURS attribution string is
                          still TBD) and water.json — the seed for the portal's map_files rows, imported once by
                          `task vm:map`. The water is this village's model: an epsilon priority-flood + D8 run on
                          ARSO DMR 1 m over E485000-488000 N44800-47600, the fill capped at 2 m because the village
                          is a closed karst basin, a course drawn from 2 ha of catchment and 250 m of length, named
                          from 100 ha. The method is the homestead repo's 10-site/terrain-data/water.py; the
                          portal draws whatever the row holds and knows none of this.
                          The picture behind the gate is porta-pagi's built-in backdrop.jpg — the owner's own
                          screenshot of a map service, provenance TBD — until this village imports its own.
deploy/infra-log.md       what was done by hand on the VM, in order — the one place allowed to be a changelog
.claude/context/village.md who uses the portal and why each room is shaped as it is
.claude/agents/reviewer.md, .claude/hooks/   the end-of-turn reviewer; every turn that changed files invokes it
```

## Rolling the village

`task roll -- sha-<commit>` writes `BE_TAG` in `.doco-cd.yml`, commits and
pushes; the VM's doco-cd reconciles within ~2 min. The tag must exist in the
`porta-pagi` package (its CI pushes `sha-<commit>` on every push to `main`
there), and **the package must be public** — the VM pulls anonymously, and a
private package leaves it on the old container with doco-cd failing quietly.
`task roll` checks the tag exists before it writes anything. A migration
in that commit runs at the roll; rehearse it on a copy of last night's backup
(`task vm:backup`) when it touches a table this village has rows in. What a
release needs from this repo — a new env variable, a CLI to run — is in the
`porta-pagi` commit subject.

The map, the codex and the backdrop are rows in the database, entered once:
`task vm:map`, `task vm:codex -- codex.json`, `task vm:backdrop -- picture.jpg`.
The codex seed stays off this repo: the repo is public and the text is the
collective's. The way back in when nobody is logged in: `task vm:code -- "<house>"`.

## Working rules

- **This repo is public.** The village name and the public GURS data are the
  only specifics in it. Nothing that identifies a person or describes a
  household comes in here: no names of people, no phone numbers, no
  house-by-house facts. History is not rewritten for a name that slipped in
  (owner's decision 2026-09-09): fix HEAD, no force-push.
- Work on `main`, commit and push (owner's rule 2026-09-07). Commits:
  lowercase, succinct; no backticks in subjects (they break the Telegram
  deploy ping).
- The VM is shared with gaias-choice: Caddy and the doco-cd controller belong
  to that repo. A change that needs a new route or a new poll entry is a
  change **there** — see `deploy/infra-log.md`.
- Docs describe current state. A manual VM step goes into `infra-log.md` in
  the same change.

## Memory

@.claude/memory/MEMORY.md
