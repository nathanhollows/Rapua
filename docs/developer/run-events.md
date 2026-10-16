---
title: "Run Events"
sidebar: true
order: 12
---

# Run Events

Every thing that happens to a run is appended to `run_events`: one immutable,
timestamped row. The state tables say what is true now. The stream says what
happened, and when, which is the question the journal, the admin's timeline, the
run lists and the research corpus all ask.

## What the stream is for

The engine's tables (`run_block_states`, `objective_context_completions`,
`section_finishes`, `runs`) are last-write-wins. They can say an objective
cleared, and cannot say how long the team was on it, how many wrong answers came
first, or that they opened it an hour before finishing it. Those are history
questions, and the stream exists to answer them.

Three properties are requirements rather than nice-to-haves, because the stream
is also study data:

- **Immutable.** Nothing in the application updates or deletes a row. The only
  removal is the cascade when a run or quest is deleted.
- **Complete.** No trimming, no sampling, no caps. (The block history cap in
  `scan_block.go` is the counter-example inside this codebase.)
- **Timestamped in UTC at full precision.** `created_at` is written from Go.
  SQLite's `current_timestamp` is second-resolution, and the gaps between events
  are the whole point.

Game rules never read the stream. Reachability, points and completion stay on
the state store, so the stream is a side channel in the read direction. In the
write direction it is a peer: where an event accompanies a state write, both go
in the caller's transaction, so anything rendered from the stream inherits the
reliability of the state it describes.

## The row

`models.RunEvent` (`models/run_events.go`), table created by
`internal/migrations/20261011000000_run_events.go`.

| Column | Type | Meaning |
|---|---|---|
| `id` | INTEGER PRIMARY KEY AUTOINCREMENT | Ordering. Two events can share a microsecond (an answer that completes its objective writes two), so the integer is the tiebreak. |
| `run_code` | VARCHAR(36) | The run it happened to. Cascades with the run. |
| `quest_id` | VARCHAR(36) | The quest, denormalised so whole-quest reads need no join. Cascades. |
| `type` | VARCHAR(48) | One of the constants below. |
| `objective_id` | VARCHAR(36) | The objective in play, where the event has one. No foreign key: deleting an objective must not erase the record of teams having visited it. |
| `block_id` | VARCHAR(36) | The block, where the event has one. No foreign key, same reason. |
| `points` | INTEGER NOT NULL DEFAULT 0 | The change this event made to the run's score. Summing the column over a run rebuilds its total. |
| `created_at` | DATETIME | When it happened, UTC, full precision. |
| `data` | JSONB | The event's payload, where it carries one. |

Indexes lead with the key and trail with the timestamp they order by:
`(run_code, created_at)` for one run's stream, `(quest_id, created_at)` for a
quest's.

**Points convention.** `block_answered` carries the credited delta (the block
computes an award, the ledger decides the credit).
`objective_context_completed` keeps 0 in the column and puts the credit in its
payload, because the award rode the answer that triggered it and counting it
twice would double the sum. Every other event carries 0.

## Event types

`models.EventType`, with payloads in `models/run_events.go` and constructors in
`internal/services/run_events.go`.

| Type | Fires when | Carries | Pace |
|---|---|---|---|
| `run_started` | a run consumes a credit and begins play | nothing | yes |
| `run_named` | a player names or renames their run | `{name}` | no |
| `objectives_listed` | a player opens the list of objectives | nothing | yes |
| `objective_viewed` | a player opens an objective they can reach | `objective_id` | yes |
| `block_answered` | a graded answer is submitted, right or wrong | `objective_id`, `block_id`, `{block_type, completed, points_awarded, submitted?}`, credited `points` | yes |
| `objective_context_completed` | an objective context clears, once per context | `objective_id`, `{context, points}` | yes |
| `section_finished` | a player chooses to end a section | `objective_id` | yes |
| `upload_added` | a run uploads media | `block_id`, `{url, media_type}` | yes |
| `notification_sent` | a message is sent to a run | `{notification_id, content}` | no |
| `notification_read` | a player dismisses a message | `{notification_id}` | no |
| `run_completed` | a run's root objective completes | nothing | yes |

Notes that matter when reading the table:

- `objective_context_completed` fires once per context, and proof and reveal
  complete separately, so one objective produces one row each. Its payload's
  `points` is the credit that rode the submission which cleared the context, zero
  on the content-only path where no submission earned anything.
- `block_answered`'s `completed` means the submission completed the block, which
  is not the same as the answer being right: only the block knows what right
  means.
- `block_answered`'s `submitted` is what the player sent when the submission was
  not a correct solve, and only for the blocks that report one
  (`game.SubmissionReporter`: password, pincode, scan, geofence, quiz, sorting,
  checklist). A geofence attempt carries the reported position, a checklist tick
  the items checked so far. A solve carries nothing, because the block's state is
  where the final answer lives.
- `notification_sent` carries the words because the `notifications` table filters
  dismissed rows out of every read, so the log is the copy of the message that
  survives a dismissal.
- `notification_read` is a dismissal, which is not the same as seeing it.
  Nothing records a sighting.
- `upload_added`'s URL may outlive the file: deleting a run removes its uploads
  and leaves the event standing.
- A reveal's completion is recorded but never drawn by the admin's log. A reveal
  zone can hold interactive blocks, so its completion lands whenever those do,
  and the block answers that cleared it draw already; the content is also on the
  page at the moment it closes. Only proof completions get a row.

## Reading a submission

`submitted` is present exactly when the submission was not a correct solve,
whatever the row's `completed` says: a quiz without retries completes on a wrong
answer, so `completed: true` with `submitted` set is a wrong answer the block
closed anyway. Absent means nothing was recorded. Present and empty
(`{"selected_options":[]}`, `{"answer":""}`) means the player submitted nothing,
which is data: a tick that cleared the last box, or a form sent without a value.

Attempts and timing come from the rows themselves. Count `block_answered` rows per
`block_id` for how many tries a block took, and read the `created_at` deltas for
how long each one took. A checklist's rows are the only record of how a team moved
through the list, since its state holds the latest set and nothing else dates the
earlier ones.

Do not drop these rows as duplicated by state. A solve's detail is recoverable
from `run_block_states`, but the timeline is not: state carries no timestamps, and
some blocks overwrite the attempt they replaced (a geofence keeps only its last
position).

Free text is left out to shape the study extract rather than to protect the
player: the response still sits in `run_block_states`.

## What is deliberately not recorded

- **Previews.** A preview writes no rows at all, and carries a run code with no
  row in `runs`, so anything written against it would trip a foreign key.
- **A resubmission to an already-complete block.** It short-circuits before any
  write, so a second click is not a second attempt.
- **A submission that fails validation.** It writes nothing; that is a client
  fault, not a player action.
- **Admin actions that are not run-scoped.** Quest edits and credit changes are
  out of scope: this is a stream about runs, not an audit log.
- **Uploads with no run.** The block editor's own media is stored without one and
  never enters a run's stream.

## Guarantees at the call sites

| Event | Written by | Guard |
|---|---|---|
| `run_started` | `RunService.StartPlaying` | inside the start transaction |
| `run_named` | `RunService.RenameRun` | inside the rename transaction |
| `objectives_listed`, `objective_viewed` | `NavigationService.RecordObjectivesListed`, `RecordObjectiveView` | own write, no state change to ride with |
| `block_answered` | `CheckInService.ValidateAndUpdateBlockState` | inside the submission transaction |
| `objective_context_completed` | `CheckInService`, on the submission that cleared the context; `recordContextCompleted` for content-only contexts | recorded only when the completion row is new |
| `section_finished` | `NavigationService.FinishSection` | recorded only when the finish row is new, so a double press gives one event |
| `upload_added` | `UploadService.UploadFile` | inside the upload transaction, only for a run's own upload |
| `notification_sent`, `notification_read` | `NotificationService` | inside their transactions |
| `run_completed` | the check-in and section-finish paths, via `recordRunCompleted` | recorded once, read inside the caller's transaction where the write lock makes the answer decisive |

## Readers

Nothing in the game engine reads the stream. These surfaces do:

| Surface | Read | Draws |
|---|---|---|
| Admin run page (`RunService.GetRunLog` → `BuildRunLog`) | `ListByRun` | the timeline, the cadence strip, the quiet stretches, the status |
| Admin run page pace | `TimesByType` over the quest's meaningful events | the quest's standing pace, used while a run is too thin for its own |
| Runs list, Activity table, leaderboard (`GetQuestRunSummaries`) | `TimesByType` once, `LatestByType` for finishes | progress and the cadence status, for every run at once |
| Player journal (`JournalService.GetJournal`) | `ListByRun` | the diary |
| Run progress (`runActivity`) | `ListByRun` | which objectives a run has touched |
| Finish guard (`RunCompletedWithTx`) | the run's `run_completed` rows | whether the run has already been ended |

## Meaningful events, and the pace

Eight types count as the team doing something: `run_started`,
`objectives_listed`, `objective_viewed`, `block_answered`,
`objective_context_completed`, `section_finished`, `upload_added`,
`run_completed`. `meaningfulTypes` in `internal/services/run_log.go` is the one
place that says so.

Excluded, each for its own reason:

- `notification_sent` and `notification_read` are driven by an admin's clock,
  and would pollute the pace with someone else's timing.
- `run_named` is a one-off that says nothing about movement.

A run's pace is the **median** of the gaps between its meaningful events, floored
at 60 seconds and needing at least four gaps before it says anything. Below that,
the run borrows the quest's median gap across all runs. Median rather than mean,
because one long gap (a lunch break, a coach ride) inflates an average and hides
the very stall it should flag.

A quiet stretch is read against that pace: past 2x is idle, 4x stalled, 8x
abandoned, each with a flat floor (5 minutes, 10 minutes, 1 hour) so a brisk game
cannot sprout idle bands from waits that are short in absolute terms. Only the
quiet still running can be abandoned: a stretch the team came back from was a
stall, however long it ran, because the walk-off never happened.

## Adding an event type

1. Add the constant and, if it carries anything, a payload struct to
   `models/run_events.go`, and add the type to `models.AllEventTypes` beside it.
   Payloads are plain data with JSON tags.
2. Add a constructor to `internal/services/run_events.go`, so call sites never
   build a type string by hand and the taxonomy cannot drift.
3. Record it at the call site with `RecordWithTx` inside the transaction that
   performs the state change, or `Record` where there is no state change to ride
   with. A failed event write fails the request: a dropped event is a silent hole
   in the journal and the corpus, and silent holes are worse in a data set than a
   loud failure.
4. Decide whether it belongs in `meaningfulTypes`, and say why in the comment.
5. Consider the readers: a new type that the run log should draw needs a case in
   `runLogEntryFor`, and one the journal should show needs a case in
   `journal_service.go`. An unhandled type is simply not drawn, which is the
   right default for noise. `TestEventTaxonomy_EveryTypeIsDecided` holds the
   decision for every type in `models.AllEventTypes`, and fails until the new
   one has a row: not drawn is a decision, and it has to be written down.
