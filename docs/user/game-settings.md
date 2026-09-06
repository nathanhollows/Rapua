---
title: "Game Settings"
sidebar: true
order: 9
---

# Game settings

There are two, and they are on the quest page.

| Setting | What it does |
|---------|--------------|
| **Points** | Blocks can award points, and teams accumulate a score. Off by default: a block's points are ignored entirely until this is on. |
| **Leaderboard** | Players can see how teams compare. Needs points to be on to mean anything. |

Everything else that shapes a game is a property of the game itself rather than
a setting sitting beside it.

## Where the rest went

**Routing** — how players are moved between things — belongs to each section,
over its own contents. A guided path in one act and open exploration in the next
is two sections with two settings, not one game-wide choice. See
[Sections](/docs/user/sections).

**How much is enough** is the completion band on a section: the least and the
most of its contents a team needs. Where a game once had one completion rule,
every section now has its own, and a range gives players a finish button instead
of advancing them automatically. See [Sections](/docs/user/sections).

**What unlocks what** is a `depends` list on an objective, naming what has to be
done first. See [Sections](/docs/user/sections).

**When the game runs** is scheduling. See
[Scheduling Games](/docs/user/scheduling-games).

## Points

Points live on blocks, not on objectives. An objective is worth the sum of what
its blocks award, so a quiz block worth 10 and a scan worth 5 make the objective
worth 15. Turning points off does not remove them from the blocks; it stops them
counting.

A game with points off still tracks what every team completed, which is what the
activity screen and each team's journal report.

## Editing a game that is running

You cannot, by default. A change reaches players immediately and cannot be taken
back: parking a section teams are working through, or removing an objective
somebody is standing at.

Stop the game to edit it, or duplicate it and work on the copy. If you mean to
edit a running game anyway, the editor has an unlock that says so plainly and
asks you to confirm.
