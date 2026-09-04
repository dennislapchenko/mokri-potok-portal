# The village — context for whoever works on the portal

A small collective in the Slovenian karst, a handful of houses, a shared
event ground north-west across the stream, and a WhatsApp group where
everything is said once and then lost. The portal is the layer under the
group: the things that must stay findable — who is away, who needs what from
the shop, when the work party is — with a game-like village map as the door.

## People and roles

- **Houses**, not persons, are the accounts. A house holds several phones.
- A house adds its **own** further phones with a six-digit pairing code from
  the Houses room. The steward is not needed for that, and it is the way back
  in after adding the portal to an iPhone home screen — the installed icon gets
  storage of its own and starts signed out.
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
| Tavern (the hall) | One room, three parts stacked: pinned notices on top, then the month calendar (kinds: event, work party, alarm, with sign-ups), then the message board. Merged 2026-09-04 — two doors to one gathering place crowded the bottom bar, and the board alone was never worth a door. `/bell` stays a route alias because notifications already sent point at it. |
| Tool shed | Who lends what, and who holds it now. A tool comes back; a give-away does not. The holder gets a nudge after a day, then at doubling gaps — a reminder, not a ledger. |
| Work bees | Any event takes sign-ups. The house that called it hears who is coming. A headcount for the day, never a score. |

| Notifications | Installed as a home-screen app (PWA), each phone can allow push. Every new post, need, give-away, run, event or away notice rings the other houses; a house switches kinds off in the Houses room. Away pushes name nothing. All six on by default is sized for a handful of houses — revisit the defaults and quiet hours when more join. |

Later, only if the houses ask: growing plans (Fields), per-house frost log
(Almanac — this site's binding constraint is frost pooling), village knowledge
(Archive), seasonal map skins.

## Things that are deliberately not here

- Renown points, quests, leaderboards. Counted reciprocity turns into debt.
- Photo uploads (v0). Money of any kind. Public pages.
- WhatsApp login via Meta's Business API — the invite link *is* the WhatsApp login.
- Offline caching in the service worker. A stale shell after a deploy costs
  more than a reload on village Wi-Fi.

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
