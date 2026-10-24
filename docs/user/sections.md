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
To start a new one inside a section, use the arrow beside **New objective** and
pick the section.
An objective's position among its siblings is the order players meet it in,
where the section above is set to a guided path.

Players are shown at most two sections deep. A section nested further still
works, but it passes through: it keeps its rules, and its tasks appear in the
section above. The editor warns about it so you can mark it pass-through on
purpose (see below).

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

Every section has a completion band: how much of what is inside a team has to
do. Open the pill in the section's header and the band reads as one sentence.

- **Players need to complete** how many: none, some, or all of them.
- **Once they have completed** that many, either the section finishes and the
  rest close with it, or they can keep going, or finish with a button. One of
  three plants, and the other two close: that is the first. Two of five stalls in
  a market, and stay as long as you like: that is the second.

When players can keep going, they see the button once they have done enough. Say
what it says: *"That's enough for now"* reads better than *"Finish"*. The section
also finishes by itself once everything is done, or sooner if you lower **They
can complete up to**.

A guided path takes every step in order, so under guided routing the band is set
to all of them and cannot be changed.

The pill shows the band as a row of dots, one for each thing in the section.
Solid dots are required, rings are what players may go on to do, and faint rings
close unplayed when the section finishes. When players finish it with the
button, a small copy of that button follows the dots, with your label on it.
Hover the band to have it said in a sentence. Click a dot to ask for that many,
or drag across the dots to let players keep going up to where you stop. A section
holding more than ten things shows the band in words instead.

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

## Pass-through

Open the pill in a section's header and turn on **Pass through**. A ghost then
leads the pill. Players never see the section as a place: its tasks are
shown in the section above, as though they belonged there. It still does its job behind the scenes, so its routing, its
completion rule and anything that depends on it work exactly as before.

The use is choosing for the player without saying how. Say a zoo trip asks each
team to study one bird, one fish and one mammal. Make a pass-through section for
each group, put the species inside, route it randomly and ask for one. Every team
sees three tasks, and a different animal in each, and nobody learns they were
drawn from a pool. Picks come from the team's code, so a team sees the same ones
every time it looks.

A few things to know:

- A section nested more than two deep is passed through whether you mark it or
  not. The ghost shows on those too, so you can see which levels players will not
  see. A section you mark does not count toward the two, so marking one lets a
  deeper level be shown.
- A pass-through section must complete on its own: choose that it finishes when
  players have done enough, not that they keep going. Otherwise its finish button
  would appear inside a section the player never entered, naming something they
  never saw.
- If it asks for fewer tasks than it shows at once, finishing makes the rest
  disappear with no explanation. Random order with a window the same size as the
  number you ask for avoids that.
- A pass-through section's own content still plays, but its name is never shown.
- The quest itself cannot be pass-through. An objective with nothing inside can
  carry the flag, but it does nothing there, and the editor warns about it.

## Keys and locks

An objective can stay locked until a team has done something somewhere else.
Interactive blocks carry the keys: give a quiz, a scan or a password a variable,
and completing it sets that variable for the team. A choice sets the variable of
whichever option the team picks. Click the lock in an objective's header to pick
the variables it waits for; it opens once every one of them is set.

Several blocks can set the same variable, so "find any one of these" needs no
section of its own: give each of them the same key. And because a single choice
sets only one option, two objectives locked on different options of the same
choice are a fork in the road: a team only ever opens one.

Content blocks can be locked too, and stay hidden until their variables are set.
A text block beside a quiz can wait on the quiz's key, and appears the moment the
answer goes in.

The editor marks what is locked, and every mark names its variables. On the
quest page a lock means the objective itself is locked; a locked page means one
of its blocks is hidden, and a key means one of its blocks sets something. Hover
any of them to see which blocks, and the rows on the other end are outlined:
everything that sets what a lock waits on, or everything waiting on what a key
sets.

A few things to know:

- Only interactive blocks set variables, and only content blocks and objectives
  wait on them. An objective's own completion never sets one: what unlocks things
  is always something a team did.
- Once set, a variable stays set. There is no way to lock something again.
- A lock nothing can ever open is flagged: a variable no block sets, or one only
  set behind the very lock waiting on it.
- A section that needs all of its contents cannot be finished if some of them are
  locked on different options of one choice. Ask for fewer, or move the branches
  into a section of their own.
- A section that finishes on its own closes everything inside it. If one objective
  is locked on a key its neighbours hold, and the section finishes before enough
  of them are done, it stays locked. The editor warns when that is likely.

## Example: a perfumer's garden

**Find a top note** (Open Exploration, 1 of 3) — a card on what a top note is,
then bergamot, lemon verbena, pink pepper. Each one's question sets `top_note`.
**Find a heart note** (Open Exploration, 1 of 3) — rose, jasmine, ylang-ylang,
each setting `heart_note`.
**Find a base note** (Open Exploration, 1 of 3) — vetiver, oakmoss, labdanum,
each setting `base_note`.
**The still room** (Open Exploration, 2 to 4) — optional, with a finish button.
**The blending bench** — locked on `top_note`, `heart_note` and `base_note`.

One from each layer, in any order, and the bench opens when a team has all three.
