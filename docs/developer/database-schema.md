---
title: "Database Schema"
sidebar: true
order: 4
---

# Database Schema

This document outlines the database schema used in Rapua. The application uses SQLite with the Bun ORM to manage and query data. Model structs live in `models/`.

Two core domain objects were renamed in 2026: `Instance` → `Quest` (table `instances` → `quests`) and `Team` → `Run` (table `teams` → `runs`), along with every table and foreign-key column that referenced them (`instance_id` → `quest_id`, `team_code`/`team_id` → `run_code`/`run_id`). This document reflects the new names; see `internal/migrations/20260801000000_quest_run_renames.go` for the exact rename mapping if you're cross-referencing old PRs or issues.

## Schema Overview

```
Quest ("quests")
├─ has-one   QuestSettings          quest_settings.quest_id
├─ has-many  Objective              objectives.quest_id
│    ├─ has-many Objective          objectives.parent_id → objectives.id  (the tree)
│    └─ has-many Block              blocks.owner_id → objectives.id
│         └─ has-many RunBlockState run_block_states.block_id
├─ has-many  Block                  blocks.owner_id → quests.id  (start and finish pages)
├─ has-many  Run                    runs.quest_id
│    ├─ has-many ObjectiveContextCompletion  objective_context_completions.run_code → runs.code
│    ├─ has-many SectionFinish      section_finishes.run_code → runs.code
│    ├─ has-many Notification       notifications.run_code → runs.code
│    └─ has-many RunBlockState      run_block_states.run_code → runs.code
└─ has-many  ShareLink              share_links.template_id  (only when is_template = true)

User ("users")
├─ has-many  Quest                  quests.user_id
└─ has-many  CreditPurchase         credit_purchases.user_id
     └─ has-many CreditAdjustments  credit_adjustments.credit_purchase_id

Referenced by quest_id / run_id / user_id but with no bun relation tag
(loaded ad hoc in repository code, not via ORM joins):
  RunStartLog, RunVarState, Upload, FacilitatorToken, ShareLink.UserID
```

## Tables

### Quest
The central table representing a game instance or template. (Was `Instance`.)

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| name | string | Name of the quest |
| user_id | string | ID of the user who owns this quest |
| is_template | bool | Whether this quest is a reusable template |
| template_id | string | ID of the template this quest was created from (if any) |
| start_time | time | When the quest is scheduled to start |
| end_time | time | When the quest is scheduled to end |
| is_quick_start_dismissed | bool | Whether the quickstart guide has been dismissed |

`Status` (Scheduled/Active/Closed) is computed from `start_time`/`end_time` via `Quest.GetStatus()` — it is not a column.

### QuestSettings
Settings that control how a quest works. (Was `InstanceSettings`.)

| Field | Type | Description |
|-------|------|-------------|
| quest_id | string | Primary key, references quests.id |
| enable_points | bool | Whether points are enabled for this game |
| show_leaderboard | bool | Whether to show the leaderboard to players |

Routing and completion are not settings on a quest: they are properties of each
objective, over its own children. See [Objective](#objective) below.

### Objective
One thing to accomplish, and the only structural type: an objective with
children is a section, one without is a leaf, and nothing else distinguishes
them. Every quest has exactly one objective with no parent, its root, which is
the quest rather than a place in it and is never rendered to a player.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| quest_id | string | Foreign key to quests.id |
| parent_id | string | The objective this sits beneath. NULL for the root, and only for the root |
| position | int | Order among siblings. Unique with parent_id, so two siblings cannot share one |
| slug | string | URL-safe slug, unique per quest |
| title | string | Name of the objective |
| color | string | Accent colour, for a section heading its children |
| draft | bool | Held out of play along with everything beneath it, without moving any of it |
| routing | string | How children are offered: `ordered`, `free_roam` or `randomised`. Inert without children |
| children_min | int | Completion band lower bound. NULL and 0 differ: see the [game spec](/docs/developer/game-spec) |
| children_max | int | Completion band upper bound |
| max_next | int | How many children a randomised objective offers at once. 0 means all |
| finish_label | string | Label for the finish button, shown only where the band has a range |

`parent_id` carries no foreign key to `objectives.id`: the rows it points at
live in the same table, and SQLite can only add a self-referencing constraint by
recreating it. Deletion cascades from quests, and the repository enforces what
the constraint would have.

### ObjectiveContextCompletion
An append-only record that a run cleared one of an objective's two contexts. The
insert is the idempotency guard: a context's `sets` fire on the call that
recorded the completion, not on every call that finds it done.

| Field | Type | Description |
|-------|------|-------------|
| run_code | string | Foreign key to runs.code |
| objective_id | string | Foreign key to objectives.id |
| context | string | `objective_proof` or `objective_reveal` |
| completed_at | time | When it cleared |

Clearing the **proof** is what completes an objective, and what the frontier,
the journal and the leaderboard all count. The reveal is the payoff afterwards,
and can sit unfinished behind an interactive block or a player who navigated
away.

### SectionFinish
An append-only record that a player pressed a section's finish button. Only a
section whose band has a range ever shows one; where min equals max there is
nothing to decide and the section completes on its own.

| Field | Type | Description |
|-------|------|-------------|
| run_code | string | Foreign key to runs.code |
| objective_id | string | Foreign key to objectives.id |
| finished_at | time | When it was pressed |

### Block
Content blocks. A block belongs to a quest's start or finish page, or to one of
an objective's two contexts.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| owner_id | string | The quest or objective this block belongs to. Polymorphic, so it carries no foreign key, which is why deletes name blocks explicitly |
| type | string | Block type identifier (e.g. `markdown`, `pincode`) — the bun tag still declares `type:int`, a leftover from when this was an int enum; the column itself holds strings |
| context | string | `start`, `finish`, `objective_proof` or `objective_reveal` |
| data | json | Block-specific data |
| ordering | int | Display order within its owner |
| points | int | Points that can be awarded for this block |
| validation_required | bool | Whether validation is required to complete this block |

No timestamps — `Block` doesn't embed the base `created_at`/`updated_at` fields that most other models do.

### Run
A team of players participating in a quest. (Was `Team`.)

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| code | string | Unique run code (player-facing) |
| name | string | Team name |
| quest_id | string | Foreign key to quests.id |
| started_at | time | When players began the run (zero until then) — distinct from `created_at`, which is when the run was provisioned |
| has_started | bool | Whether the team has started the game |
| points | int | Total points earned by the team |

`VarStates` (creator-defined variable values, keyed by name) is populated by `RunService`, not a column — see `RunVarState` below for the backing table.

### RunBlockState
Tracks the state of blocks for each run. (Was `TeamBlockState`.)

| Field | Type | Description |
|-------|------|-------------|
| run_code | string | Part of composite primary key, references runs.code |
| block_id | string | Part of composite primary key, references blocks.id |
| quest_id | string | Part of composite primary key — added when the table gained a third PK column during the rename; previously a 2-column key |
| is_complete | bool | Whether the team has completed this block |
| points_awarded | int | Points awarded to the team for this block |
| player_data | json | Player-specific data for this block |

### Notification
Messages sent to teams during gameplay.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| content | string | Notification content |
| type | string | Notification type |
| run_code | string | Foreign key to runs.code |
| dismissed | bool | Whether the notification has been dismissed |

### User
User accounts for game administrators.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| name | string | User's name |
| display_name | string | Optional display name |
| email | string | User's email (unique) — also tagged as a primary key alongside `id`; that looks unintentional (all repository lookups key off `id` alone) and is worth confirming with whoever touches this struct next |
| email_verified | bool | Whether the email has been verified |
| email_token | string | Token for email verification |
| email_token_expiry | time | When the email token expires |
| password | string | Hashed password |
| provider | string | Auth provider — `google`, or empty string for email/password |
| share_email | bool | Whether the user has opted in to sharing their email (e.g. with template authors) |
| work_type | string | Optional free-text description of the user's role/sector |
| free_credits | int | Current free credit balance |
| paid_credits | int | Purchased credit balance |
| monthly_credit_limit | int | Monthly free-credit allocation |
| stripe_customer_id | string | Stripe customer ID, if any |

`CurrentQuestID`/`CurrentQuest` used to be a persisted column but now live in session storage instead — the struct fields remain for convenience but are not DB columns.

### CreditPurchase
A record of a credit purchase made via Stripe. New table — part of the billing subsystem.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| user_id | string | Foreign key to users.id |
| credits | int | Number of credits purchased |
| amount_paid | int | Amount paid, in cents |
| stripe_payment_id | string | Stripe payment ID |
| stripe_session_id | string | Stripe checkout session ID (unique) |
| stripe_customer_id | string | Stripe customer ID |
| receipt_url | string | Link to the Stripe receipt |
| status | string | `pending`, `completed`, `failed`, or `cancelled` |

### CreditAdjustments
A manual or automated adjustment to a user's credit balance (top-ups, admin grants, migrations). New table.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| user_id | string | Foreign key to users.id |
| credits | int | Credit delta applied |
| reason | string | Human-readable reason, prefixed with one of `Migration`, `Monthly free credit top-up`, `Purchase`, `Admin`, `Gift` |
| credit_purchase_id | string | Foreign key to credit_purchases.id, if this adjustment resulted from a purchase |

Struct name is plural (`CreditAdjustments`) while every sibling model is singular — an inconsistency, but used that way consistently across the codebase, so not a typo to "fix" casually.

### RunStartLog
Audit log of when a user started a run. (Was `TeamStartLog`.)

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| user_id | string | Foreign key to users.id — who started the run |
| quest_id | string | Foreign key to quests.id |
| run_id | string | Foreign key to runs.id |

### RunVarState
Stores creator-defined variable values for a run within a quest. (Was `TeamVarState`.)

| Field | Type | Description |
|-------|------|-------------|
| run_code | string | Part of composite primary key, references runs.code |
| quest_id | string | Part of composite primary key, references quests.id |
| var_name | string | Part of composite primary key — the variable's name |
| var_value | string | The variable's current value |

Surfaced on `Run.VarStates` at runtime; not a bun relation.

### FacilitatorToken
Tokens that allow facilitators to access a quest, optionally scoped to specific objectives.

| Field | Type | Description |
|-------|------|-------------|
| token | string | Primary key, unique token |
| quest_id | string | Foreign key to quests.id |
| objectives | string[] | JSON-encoded list of objective IDs this token is restricted to (empty means unrestricted) |
| expires_at | time | When the token expires |

Note there is no `created_by` column on this table (unlike `ShareLink`).

### ShareLink
Links that allow sharing templates.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| template_id | string | Foreign key to quests.id (where is_template = true) |
| user_id | string | Owner of the link |
| expires_at | time | Optional expiry |
| max_uses | int | Maximum number of uses; 0 means unlimited |
| used_count | int | Number of times the link has been used |
| regenerate_codes | bool | Whether importing via this link regenerates run codes |

There is no longer a `name` field on share links.

### Upload
Uploaded media files.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| original_url | string | Link to the original uploaded file |
| timestamp | time | When the file was uploaded |
| quest_id | string | Quest the upload belongs to, if any |
| run_code | string | Run the upload is attached to, if any |
| block_id | string | Block the upload is attached to, if any |
| storage | string | Storage backend identifier |
| delete_data | string | Data needed to delete the file from storage |
| type | string | `image` or `video` |
| sizes | json | Unexported field holding a JSON-encoded list of resized image variants (breakpoint + URL), accessed via `GetSizes()`/`AddSize()` |

This table's shape changed substantially beyond the rename — it no longer has `user_id`, `filename`, `size`, or `content_type` columns.

## Key Relationships

1. **Quest to Objectives**: One-to-many. Every quest has exactly one objective with no parent, its root.
2. **Objective to Objectives**: One-to-many, via `parent_id`. This is the tree; there is no separate group type.
3. **Objective to Blocks**: One-to-many, via `owner_id`. A block belongs to one of an objective's two contexts.
4. **Quest to Blocks**: One-to-many, also via `owner_id`, for the start and finish pages. `owner_id` is polymorphic and carries no foreign key, which is why deletes name blocks explicitly rather than leaving them to cascade.
5. **Quest to Runs**: One-to-many. Each quest has multiple runs (teams).
6. **Run to ObjectiveContextCompletions**: One-to-many. What a run cleared, append-only.
7. **Run to SectionFinishes**: One-to-many. Which sections a player chose to end.
8. **Run to RunBlockState**: One-to-many. Runs have state for each block they interact with.
9. **User to Quests**: One-to-many. Users can create multiple quests.
10. **Quest to Template**: Many-to-one, via `template_id`. Many quests can be created from one template.
11. **Template to ShareLinks**: One-to-many. A template can have multiple share links.
12. **User to CreditPurchases**: One-to-many. Purchases feed balance adjustments via `CreditAdjustments`.

## Database Indexes

Beyond primary keys, notable explicit indexes (from `internal/migrations/`) include:

- `objectives_quest_slug` — unique, on `objectives (quest_id, slug)`
- `idx_objectives_parent_position` — unique, on `objectives (parent_id, position)`, so two siblings cannot share a position. SQLite checks it per statement rather than at commit, which is why every renumber parks its rows clear before settling them
- `idx_objectives_parent_id` on `objectives (parent_id)`, `idx_objectives_quest_id` on `objectives (quest_id)`
- `idx_objective_context_completions_run_code` on `objective_context_completions (run_code)`
- `idx_blocks_owner_id` on `blocks (owner_id)`
- `idx_teams_instance_id` on `runs (quest_id)`, `idx_teams_id` on `runs (id)`
- `idx_notifications_team_code` on `notifications (run_code)`
- `idx_blocks_owner_id` on `blocks (owner_id)`
- `idx_uploads_instance_id`, `idx_uploads_team_code`, `idx_uploads_block_id` on `uploads`
- `idx_facilitator_tokens_instance_id` on `facilitator_tokens (quest_id)`
- `idx_share_links_template_id`, `idx_share_links_user_id` on `share_links`
- `idx_credit_purchases_user_id`, `idx_credit_purchases_stripe_session_id`, `idx_credit_purchases_status` on `credit_purchases`
- `idx_credit_adjustments_user_id`, `idx_credit_adjustments_purchase_id` on `credit_adjustments`
- `idx_team_start_log_user_id`, `idx_team_start_log_instance_id` on `run_start_logs`

Index **names** were not updated by the quest/run rename (`ALTER TABLE ... RENAME` doesn't rename dependent indexes) — several of the names above still say `instance`/`team` even though they now index `quest_id`/`run_code` columns on the renamed tables. Don't assume an index name reflects the current table/column name; check `internal/migrations/20260329000000_cascade_deletes.go` and `20260801000000_quest_run_renames.go` if in doubt.

## Enumerations

The database uses several enum types, most now implemented as strings rather than bare integers:

1. **RouteStrategy** (string: `ordered`, `free_roam`, `randomised`) — how an objective offers its children. Stored per objective, meaningless on one with no children.
2. **BlockContext** (string: `start`, `finish`, `objective_proof`, `objective_reveal`) — which surface a block belongs to. The two objective contexts are the proof, which gates, and the reveal, which follows it.
3. **GameStatus** (int: `Scheduled`, `Active`, `Closed`) — computed on `Quest` from `start_time` and `end_time`, not stored. Editing is refused while a quest is `Active`.
4. **Provider** (string: `google`, or `""` for email/password) — a user's auth provider.

Completion is not an enum any more. Where a `CompletionType` of `all` or
`minimum` once described a group, an objective now carries a band over its
children: `children_min` and `children_max`, both nullable, where omitting both
means every child. See the [game spec](/docs/developer/game-spec).

Dropped since the previous version of this document: `Location`, `Marker`,
`CheckIn` and `Clue` (removed entirely), the `quests.game_structure` blob and
its `GameStructure`/`CompletionType` types (the tree is `objectives.parent_id`
and `position` now), `runs.skipped_group_ids`, `locations.when_clause` (replaced
the `secret` route strategy, and
`quest_settings.show_team_count`.
