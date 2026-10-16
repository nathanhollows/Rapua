---
title: "Comment Guidelines"
sidebar: true
order: 16
---

# Comment Guidelines

How comments are written in this codebase. The rules are short; the examples are
the point.

## Say why, not what

A comment earns its place by saying something the code cannot. "Increment the
counter" is noise above `counter++`. A reason, a constraint, a piece of history,
or the thing that would break if the line changed: that is a comment.

```go
// No foreign key: deleting an objective must not erase the record of teams
// having visited it.
ObjectiveID string `bun:"objective_id"`
```

```go
// Increment the counter.
counter++
```

When a function's name and body already say the thing, delete the comment
rather than reword it. Deletions are the common case.

## Exported comments start with the symbol

`// BuildRunLog turns a run's recorded events into the admin's view of it.` Not
"Builds the run log" and not "This function...". The first word is the name, so
`godoc` reads as a sentence.

## Punctuation and shape

- End every comment with a full stop. The `godot` linter enforces it. Directives
  (`//go:build`, `//nolint:thing // reason`) are exempt, and a `nolint` gets its
  reason after the directive.
- No em dashes. Use a colon, a full stop, parentheses, or a comma. The dash is a
  house-style tell and it is banned outright.
- One line where one line will do. If the why needs a second sentence, let the
  first line stand as the summary and the second carry the detail.
- Explain a decision where the decision is, not in a block comment at the top of
  a file that the reader has already scrolled past.

## Never refer to stages or plans

Comments describe what is true now. They do not describe a route to somewhere
else, because the reader has no way to know which step they are standing on:

- no "stage 2", "phase one", "step 3";
- no "temporary until", "for now", "will be replaced later", "once X lands";
- no "not yet wired", "TODO when the migration finishes".

If something is deliberately incomplete, say what it is and what it does today.
A comment that will be wrong tomorrow is worse than no comment.

## What a good one looks like

```go
// The axis runs to the last band's drawn end: shorter than the logical one
// when the tail was capped, and equal to it otherwise.
```

```go
// Median, not mean: one long gap, a lunch break or a coach ride, inflates an
// average and hides the very stall it should flag.
```

Both say why the code is shaped the way it is, in plain words, without
describing the mechanics a reader can see.

## Comments do not cover for bad names

If a comment has to explain what a variable holds, rename it. If a comment has
to explain what a function does, the function is doing too much or is badly
named. Comments carry reasons; names carry facts.
