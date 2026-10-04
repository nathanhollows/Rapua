---
title: "Navigation Logic Reference"
sidebar: true
order: 6
---

# Navigation Logic Reference

How the game decides what a run can do next.

Nothing here is stored. Every status is derived from recorded fact each time it
is asked for, because two stored facts about the same run can disagree and a
derivation from them cannot.

---

## The frontier

The frontier is every objective's status for one run:

| Status | Meaning |
|--------|---------|
| `locked` | Out of reach. Something above it is unfinished, or its parent's routing has not offered it |
| `available` | The run can work on it now |
| `finishable` | Available, and its band has a range whose minimum is met: the player may finish it or carry on |
| `complete` | Done, and its branch is closed |

It is derived in two passes, because the two questions face opposite
directions. **Completion rises**: an objective is complete once its own proof
has cleared and enough of its children are complete, so children settle first.
**Reachability descends**: an objective is reachable only if everything above it
is open, so ancestors settle first.

The frontier spans branches at different depths, deliberately. A run can have
work available in several places at once.

## What completes an objective

Clearing its **proof** context, and meeting its band over its children.

- A leaf has no band, so its proof is the whole of its completion. Having
  nothing to prove is not the same as being finished: a leaf with no proof
  blocks still needs a completion row, or a run could finish it without ever
  seeing it.
- A section's proof gates its children as well as itself. Nothing beneath it is
  reachable until its proof clears.

The reveal is the payoff afterwards. It can sit unfinished — behind an
interactive block, or a player who navigated away — while the objective is done.
Everything that counts completion counts the proof: the frontier, the journal,
the outstanding-work list and the leaderboard.

## Routing

An objective's `routing` decides which of its children it offers:

| Strategy | Offers |
|----------|--------|
| `ordered` | One at a time, in position order. Every earlier sibling must be complete |
| `free_roam` | All of them |
| `randomised` | A window of `max_next`, shuffled deterministically from the run code so a team sees a stable order across requests |

Routing governs every child alike, sections included. A section offered this way
then routes its own contents by its own rule.

## The completion band

`children_min` and `children_max` over an objective's children. Both optional,
and the pair is filled before any rule reads it:

- Omitting both means every child: `[n, n]`.
- Naming either bound widens the other to its extreme — min to 0, max to the
  child count — which is why an explicit `children_min: 0` is a different node
  from an omitted one.
- Where min equals max there is nothing to decide, and the objective completes
  on its own at that count.
- Where min is lower, reaching min offers a finish button, and **the press** is
  what completes the objective. It also completes on its own at max.

The band counts **published** children. A drafted child can never earn a
completion row, so counting it would be a band no run could meet.

## What is in play

Drafting gates an objective and everything beneath it, without marking any of
them: the flag stays where the author set it, so publishing restores exactly
what was there.

That makes "is this in play" a question about ancestry rather than about a
column, and one function answers it — `navigation.InPlay`. The frontier prunes
for itself, because it needs both views: a section whose children are all parked
looks childless once pruned, and it is not a leaf unless the full tree says so.

Completion is derived over **every** row, drafts included, because it records
what a run did rather than what it can still reach. An objective cleared before
it was parked stays cleared, so the gates it opened stay open.

## Reachability is a gate, not a filter

A list that merely leaves an objective out is not a gate: a guessed slug, a
printed QR code or a stale bookmark reaches the page directly. So the page asks
the frontier for the objective's status rather than re-deriving one piece of it,
and every reason the list would omit an objective is a reason the page turns it
away.

## Implementation Reference

- **Frontier**: `navigation/frontier.go`
- **Loading a run's state**: `internal/services/run_state_loader.go`
- **Service**: `internal/services/navigation_service.go`
- **Grammar and lint rules**: [Game Spec](/docs/developer/game-spec)
- **Tests**: `navigation/frontier_test.go`, which includes the perfumers
  conformance shape
