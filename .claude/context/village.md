# The village — context for whoever works on the portal

A small collective in the Slovenian karst, a handful of houses, a shared
event ground north-west across the stream, and a WhatsApp group where
everything is said once and then lost. The portal is the layer under the
group: the things that must stay findable — who is away, who needs what from
the shop, when the work party is — with a game-like village map as the door.

## People and roles

- **Houses**, not persons, are the accounts. A house holds several phones.
- **Stewards** create houses, hand out invite links, assign parcels on the
  map, pin posts. One steward house today (the owner's); a second one in the
  village is the exit plan. The first steward founds the village with the
  bootstrap code from the server log.
- The owner of this repo is one house and the first steward, remote part of
  the week. A second steward in the village is the exit plan.

## The rooms and the job each one does

| Room | Job it does better than the chat |
| --- | --- |
| Village map (home) | Who lives where. Parcels coloured by house crest; the event ground is a house of kind `common`. |
| Watchtower | "We are away from … to …, please watch the land." Dates, care notes, one watcher. The room that justifies the app. |
| Market | "Going to the shop, anyone need anything?" Needs with states, give-aways, shop runs with a cut-off. |
| Bell tower | Every village event in one place; kinds: event, work party, alarm. |
| Tavern | Pinned notices and threads that must stay findable. Smallest room, first to cut. |

Later, only if the houses ask: growing plans (Fields), shared tools (Tool
shed), per-house frost log (Almanac — this site's binding constraint is frost
pooling), village knowledge (Archive), web push for alarms.

## Things that are deliberately not here

- Renown points, quests, leaderboards. Counted reciprocity turns into debt.
- Photo uploads (v0). Money of any kind. Public pages.
- WhatsApp login via Meta's Business API — the invite link *is* the WhatsApp login.

## Map data

`frontend/public/data/parcels.geojson`: ~500 parcels of k.o. 1590, public
cadastre WFS snapshot 2026-08-15, EPSG:3794 metres, rounded to 0.1 m. The
GURS open-data licence string is not recorded yet — it must be, before the
portal moves to the collective's own domain. Drawn as
SVG with northing flipped; no tiles. `channels.json` is a D8 flow model of the
plot's own window only — dashed, labelled as a model, not a survey.

## Where the thinking lives

The design plan, the brainstormed rooms, the privacy reasoning and the exit
plan are in the owner's homestead repo at `70-collective/village-app/plan.md`.
This file is the summary; that one is the argument.
