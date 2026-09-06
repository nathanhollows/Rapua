---
title: "Sections"
sidebar: true
order: 12
---

# Sections

A section is an objective with other objectives inside it. There is no separate
kind of thing to create: any objective becomes a section the moment you put
something under it, and stops being one when you take the last thing out.

That means a section can carry content of its own. "Find a heart note" can
explain what a heart note is before offering the three plants beneath it, and
players see that card first — the section holds their attention until they have
read it, then steps back and its children take its place.

Use sections for anything with parts:

- An act in a story
- A phase of a scavenger hunt
- A themed area of a tour
- A category where one of several will do

## Order and nesting

Drag an objective onto another to put it inside. Drag it out to lift it back up.
An objective's position among its siblings is the order players meet it in,
where the section above is set to a guided path.

Nesting more than four deep is allowed but warned about: past that there is more
context to hold than a phone screen shows.

## Routing

How a section offers what is inside it:

- **Guided Path** — one at a time, in order. Nobody skips ahead.
- **Open Exploration** — all of them, any order.
- **Randomised Route** — a few at a time, chosen for each team and stable for
  them. Set how many with `max_next`. Good for spreading a crowd out.

Routing applies to everything directly inside the section, whether those are
plain objectives or more sections. A section chosen this way then routes its own
contents by its own rule.

## How much is enough

Every section has a completion band: the least and the most of its contents a
team needs.

- **Leave both blank** and every one is required.
- **Set them equal** — say 1 and 1 — and the section finishes the moment that
  many are done. One of three plants, and the other two close with it.
- **Set a range** — say 2 and 5 — and reaching the lower number offers players a
  finish button. The section is not done until they press it, so they can carry
  on if they want to. Name the button with a finish label: *"That's enough for
  now"* reads better than *"Finish"*.

The band counts what is in play. Drafting one of the contents lowers what the
section can reach, and the editor refuses a draft that would put an explicit
minimum out of reach.

## Drafts

Any objective can be a draft. A drafted objective is hidden from players along
with everything inside it, keeps its place, and comes back exactly where it was
when you publish it again.

New objectives start as drafts, so you can build a section before anyone can see
it. Nothing inside a draft is marked, so publishing the section restores whatever
was under it, drafts included.

The quest itself cannot be a draft: that would take the whole game out of play.

## Unlocking on something else

An objective can wait for another. Give it a `depends` list naming what has to
be done first, and it stays locked until all of them are.

This is what makes "one of each" possible. Three categories, each needing one of
its own contents, and a final objective depending on all three: the choice lives
in the sections, and the requirement lives in the list.

A depends naming a draft is flagged — the name resolves, so nothing looks wrong,
but no team can complete it and the gate never opens.

## Example: a perfumer's garden

**Find a top note** (Open Exploration, 1 of 3) — a card on what a top note is,
then bergamot, lemon verbena, pink pepper.
**Find a heart note** (Open Exploration, 1 of 3) — rose, jasmine, ylang-ylang.
**Find a base note** (Open Exploration, 1 of 3) — vetiver, oakmoss, labdanum.
**The still room** (Open Exploration, 2 to 4) — optional, with a finish button.
**The blending bench** — depends on all three categories.

One from each layer, in any order, and the bench opens when a team has all three.
