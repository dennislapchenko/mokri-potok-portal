# Later: what is written down and not built

Recorded 2026-09-07, four days after the first commit. This file is the one
home for everything the portal does **not** do yet: rooms that were
brainstormed, variants that were designed and set aside, decisions still open,
and ideas pondered for a tight-living community. What *is* built lives in
`README.md` (rooms and stack), `.claude/context/village.md` (the villager job
each room serves) and `CLAUDE.md` (the invariants). Nothing here is a promise.

**The rule that governs this whole file**: no room ships before the houses ask
for it, after about four weeks of using what exists. Every row below is a
hypothesis until a house says otherwise, in the Tavern itself.

## Designed and set aside

Each of these has a written argument. Build from the argument, never from
memory.

| Item | Where the argument is | What it would cost |
| --- | --- | --- |
| `houses.left_at`, the soft ending of a stay: devices and invite revoked, the row kept and flagged, faded to the bottom of the Houses room, reversible by a steward | `design-membership.md` § Off-season, and ending a stay | One column, a handful of filters. Replaces deletion as the exit only if a deletion ever hurts |
| A "Commons" super-room holding Projects, Campground and a future Archive behind one tile | `design-more-rooms.md` § Navigation, option F | One more tap to every room in it. Earns its place only when Home tiles pass about eight |
| Task due reminders, read off `due_at` and one `reminded_at`, like tool returns | `design-more-rooms.md` § Push | A second nudge loop in `remind.go`. No counter, ever |
| Steward button that clears camp labels older than 12 months | `design-more-rooms.md` § Privacy | One endpoint, one button. Waits for the privacy note |
| Narrowing "any house may edit an event" to the creator and stewards | `CLAUDE.md`, provisional invariant | Waits for a reduced viewer role, which waits for volunteers who are not villagers |
| Slovenian case in the run banners. A destination is free text a house typed, so "v " + text prints "v Ribnica" where "v Ribnico" is correct. All three banners have it — the run created, the run moved, the run called off | `server.go`, the run notifications | The case-free form is "smer Ribnica" ("vožnja smer Ribnica"), applied to all three at once so they still read alike. Three strings and their pins in `banner_test.go` and `TestMarketEdits` |

## Brainstormed rooms, ranked by how often a village would reach for them

Carried over from the owner's planning notes, abstracted. Version labels are
the original guess, not a schedule.

| Idea | Building | Why | Version | Risk |
| --- | --- | --- | --- | --- |
| Seed and seedling exchange, harvest surplus | Market, a seasonal shelf | Spring and August peaks. Same shelf as give-aways with a "seeds" tag | v1 | Low |
| Bulk orders: firewood, feed, gravel, insulation | Market, a "joint order" post | One delivery, shared freight. **The app records the order, never the money** | v1 | Money among neighbours. A list, not a ledger |
| Visitors board: who hosts guests this weekend | Watchtower | Dogs, gates, strangers on the path | v1 | Low |
| Photo wall, a chronicle | Tavern | The village sees itself grow. Photos already live in the database as BLOBs | v1 | Storage grows with the years |
| Seasonal map skin | Map | The game feel. The map changes with the real season | v1 | Low |
| Fields: growing plans per house | New room | What grows where, what is planted when | v1 | Only if houses garden together |
| Almanac: per-house frost log | New room | Frost pooling is this site's binding constraint. A shared record of first and last frost per place is worth more than any forecast | v1 | Needs a habit, or it stays empty |
| Archive: village knowledge base. Valves, pipes, who to call, how the water system works | New room | Infrastructure knowledge lives in a few heads | v2 | Needs a steward or it rots |
| Skills board: who welds, who has a chainsaw licence | Archive, opt-in cards | Useful the day something breaks | v2 | Privacy. Opt-in only, each house writes its own |
| Duty roster: mowing common ground, snow, water system | Tavern, recurring | Only if the collective has duties | v2 | Builds resentment if the roster is not the collective's own idea |
| WhatsApp one-time-code login | Gate | Replaces the invite link | v2 or never | Meta's Business API, a per-verification fee |
| Moving into the collective's own website | Deploy | One address for everything | v2 or never | The collective asks for it |
| Renown points, quests, leaderboards | — | **Never, unless the houses ask.** Counted reciprocity turns into debt | — | High |

## Pondered 2026-09-07: what a tight-living community could still want

Ordered by how much each would beat "scroll up in the WhatsApp group". Each
respects the invariants: no tallies, no money, nothing public, nothing on the
cadastre about who sleeps where.

1. **Cold pools on the map.** The priority-flood that found the streams also
   finds the sinks, and a sink is where frost settles. One more JSON file from
   the same terrain pipeline, one more component beside `Water.tsx`, and the
   Almanac has a map before it has a table.
2. **One real thermometer.** The forecast panel is a town away and must never
   become a frost source. A cheap sensor at the cold pool posting to the backend
   would be the one measurement the village owns. It turns the Almanac from a
   diary into an instrument. Hardware, so a separate decision.
3. **State of the commons.** Pump on or off, road passable, gate broken, event
   ground mown. Each shared thing has a current state, a last note and who wrote
   it. It is a need that never closes.
4. **Knowledge pinned to a place.** The Archive as map pins: the main valve, the
   hydrant, the septic lid, the meter. A newcomer taps the map instead of asking
   who remembers. Infrastructure on the map is not tenancy on the map, so the
   cadastre rule holds.
5. **Who is here.** The Watchtower says who is away. The inverse, who is in the
   village this weekend and who hosts guests, answers the dog, the gate and the
   stranger on the path. Same privacy line, same logged-in app, and it folds
   the visitors board into a room that exists.
6. **The year's wheel.** Chimney sweep, winterising the water system, the spring
   clean of the event ground. A recurring event with no assignee. The roster
   risk disappears when nobody is named.
7. **A thing for a day.** A shared trailer or the good chainsaw booked for
   Saturday. The tool shed already knows who holds a thing. A date on the loan
   is one column.
8. **Wildlife log.** "Bear seen by the upper road, Tuesday evening." Not an
   alarm, because an emergency is a phone call. A dated list is local knowledge
   that otherwise lives in a few heads.
9. **Consent, not votes.** Wish options are findings and nothing ranks them. A
   proposal room could work the same way: a thread where only a reasoned
   objection is a state, and no objection after a date means agreed. This is a
   governance decision for the village to make in a room, not a feature to
   ship quietly.
10. **The village book.** The export already holds every post, event and photo.
    A yearly PDF from it is a chronicle nobody had to write.

The best next feature this week is none of these. It is a pinned post in the
Tavern asking the houses what they miss, and then four weeks of listening.

## Open decisions that block nothing today

| Decision | Who answers | Recorded where once decided |
| --- | --- | --- |
| GURS open-data licence string for `parcels.geojson` | The owner, from the GURS terms | `CLAUDE.md` § Cadastre, `village.md` § Map data |
| Provenance of `backdrop.jpg` | The owner | `village.md` § Deliberately not here |
| Treasurer, and who camp money is handed *to* | The collective | `CLAUDE.md` § Campground |
| The one-page privacy note, with a line about campers | The collective's board, a lawyer confirms | A new `docs/privacy.md` |
| A Slovenian-speaking house checks the room names and the Houses subtitle | Any house | `i18n.tsx` |
| The all-kinds-on push default once more houses join | Every house, after a week | `village.md` § Notifications |
| A second steward | The village | `village.md` § People and roles |
| What a household without land calls itself | The household, when the first one joins | `design-membership.md` |
| Whether editing the Codex in the app *amends* the adopted text or *proposes* an amendment for the council. Today any house edits and the section names it; the first edit replaces the council's wording with no copy kept but the nightly backup. A proposal state, or a section history, waits for the village to say it wants one | The collective | `CLAUDE.md` § Codex |
| Reordering codex sections in the app. New sections append; the order the import set is the document's | Any house, if a new section ever belongs in the middle | `Codex.tsx`, one `ord` field |
| The two languages of the adopted text do not carry the same number of principles in one section. Which one is the text | The collective | The codex itself, in the app |
