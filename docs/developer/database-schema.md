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
├─ has-many  Location               locations.quest_id
│    ├─ has-one  Marker             locations.marker_id → markers.code
│    └─ has-many Block              blocks.owner_id → locations.id
│         └─ has-many RunBlockState run_block_states.block_id
├─ has-many  Run                    runs.quest_id
│    ├─ has-one  Location           runs.must_scan_out → locations.id  ("BlockingLocation")
│    ├─ has-many CheckIn            check_ins.run_code → runs.code
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
| game_structure | json | Hierarchical location-grouping structure — see [Embedded Structures](#embedded-structures) below |

`Status` (Scheduled/Active/Closed) is computed from `start_time`/`end_time` via `Quest.GetStatus()` — it is not a column.

### QuestSettings
Settings that control how a quest works. (Was `InstanceSettings`.)

| Field | Type | Description |
|-------|------|-------------|
| quest_id | string | Primary key, references quests.id |
| must_check_out | bool | Whether players must check out of a location before moving on |
| show_team_count | bool | Whether to show the number of teams at each location — name predates the Team→Run rename and was not updated |
| enable_points | bool | Whether points are enabled for this game |
| show_leaderboard | bool | Whether to show the leaderboard to players |

Navigation-mode and completion-method settings that used to live here were dropped (`20260425000000_drop_navigation_modes.go`); routing/completion is now configured per location group inside `Quest.game_structure` — see [Navigation Logic Reference](/docs/developer/navigation-logic).

### Location
A location or station in a game.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| name | string | Name of the location |
| slug | string | URL-safe slug, unique per quest (partial unique index, see [Indexes](#database-indexes)) |
| quest_id | string | Foreign key to quests.id |
| marker_id | string | Foreign key to markers.code |
| criteria | string | Criteria for unlocking this location |
| when_clause | text | Optional visibility condition, stored as a `when` clause JSON object |
| order | int | Order in which this location appears |
| total_visits | int | Total number of team visits |
| current_count | int | Current number of teams at this location |
| avg_duration | float | Average time teams spend at this location |
| points | int | Points awarded for visiting this location |

### Marker
Physical markers that players scan to check into locations.

| Field | Type | Description |
|-------|------|-------------|
| code | string | Primary key, unique location code (typically 5 characters) |
| lat | float | Latitude coordinate |
| lng | float | Longitude coordinate |
| name | string | Name of the marker |
| total_visits | int | Total number of visits to this marker |
| current_count | int | Current number of teams at this marker |
| avg_duration | float | Average time teams spend at this marker |

### Block
Content blocks that make up a location's (or other owner's) interactive elements.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| owner_id | string | ID of the owning entity — currently always a location, but generalized from `location_id` to allow other owners in future |
| type | string | Block type identifier (e.g. `markdown`, `pincode`) — the bun tag still declares `type:int`, a leftover from when this was an int enum; the column itself holds strings |
| context | string | Which context the block occupies (e.g. location content vs. navigation) |
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
| must_scan_out | string | Marker/location id the team must scan to check out (if any) |
| points | int | Total points earned by the team |
| skipped_group_ids | string[] | Location-group IDs the team has skipped |

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

### CheckIn
Records when teams check in and out of locations.

| Field | Type | Description |
|-------|------|-------------|
| run_code | string | Part of composite primary key, references runs.code |
| location_id | string | Part of composite primary key, references locations.id |
| quest_id | string | Foreign key to quests.id |
| time_in | time | When the team checked in |
| time_out | time | When the team checked out |
| must_check_out | bool | Whether check-out is required |
| points | int | Points awarded for this check-in |
| blocks_completed | bool | Whether all blocks at this location have been completed |

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
Tokens that allow facilitators to access game instances, optionally scoped to specific locations.

| Field | Type | Description |
|-------|------|-------------|
| token | string | Primary key, unique token |
| quest_id | string | Foreign key to quests.id |
| locations | string[] | JSON-encoded list of location IDs this token is restricted to (empty means unrestricted) |
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
| location_id | string | Location the upload is attached to, if any |
| quest_id | string | Quest the upload belongs to, if any |
| run_code | string | Run the upload is attached to, if any |
| block_id | string | Block the upload is attached to, if any |
| storage | string | Storage backend identifier |
| delete_data | string | Data needed to delete the file from storage |
| type | string | `image` or `video` |
| sizes | json | Unexported field holding a JSON-encoded list of resized image variants (breakpoint + URL), accessed via `GetSizes()`/`AddSize()` |

This table's shape changed substantially beyond the rename — it no longer has `user_id`, `filename`, `size`, or `content_type` columns.

## Embedded Structures

### GameStructure (`quests.game_structure`)
Not a table — a JSON blob stored on `Quest.game_structure` (via `sql.Scanner`/`driver.Valuer`) representing the hierarchical tree of location groups for a quest. Every quest has exactly one invisible root group; visible groups are its `SubGroups`, each with their own `Routing` (`RouteStrategy`), `CompletionType`, and optional `When` visibility clause. This is the mechanism that replaced the old flat `criteria`/`completion` fields and the dropped navigation-mode settings.

See [Navigation Logic Reference](/docs/developer/navigation-logic) for how routing and completion are evaluated.

## Key Relationships

1. **Quest to Locations**: One-to-many. Each quest has multiple locations.
2. **Location to Blocks**: One-to-many, via `owner_id`. Each location has multiple content blocks.
3. **Quest to Runs**: One-to-many. Each quest has multiple runs (teams).
4. **Run to CheckIns**: One-to-many. Runs can check in to multiple locations.
5. **Location to Marker**: Many-to-one. Multiple locations can use the same marker.
6. **Run to RunBlockState**: One-to-many. Runs have state for each block they interact with.
7. **User to Quests**: One-to-many. Users can create multiple quests.
8. **Quest to Template**: Many-to-one, via `template_id`. Many quests can be created from one template.
9. **Template to ShareLinks**: One-to-many. A template can have multiple share links.
10. **User to CreditPurchases**: One-to-many. Purchases feed balance adjustments via `CreditAdjustments`.

## Database Indexes

Beyond primary keys, notable explicit indexes (from `internal/migrations/`) include:

- `locations_instance_slug` — unique, on `locations (quest_id, slug) WHERE slug != ''`
- `idx_locations_instance_id` on `locations (quest_id)`, `idx_locations_marker_id` on `locations (marker_id)`
- `idx_teams_instance_id` on `runs (quest_id)`, `idx_teams_id` on `runs (id)`
- `idx_check_ins_instance_id` on `check_ins (quest_id)`
- `idx_notifications_team_code` on `notifications (run_code)`
- `idx_blocks_owner_id` on `blocks (owner_id)`
- `idx_uploads_instance_id`, `idx_uploads_team_code`, `idx_uploads_block_id`, `idx_uploads_location_id` on `uploads`
- `idx_facilitator_tokens_instance_id` on `facilitator_tokens (quest_id)`
- `idx_share_links_template_id`, `idx_share_links_user_id` on `share_links`
- `idx_credit_purchases_user_id`, `idx_credit_purchases_stripe_session_id`, `idx_credit_purchases_status` on `credit_purchases`
- `idx_credit_adjustments_user_id`, `idx_credit_adjustments_purchase_id` on `credit_adjustments`
- `idx_team_start_log_user_id`, `idx_team_start_log_instance_id` on `run_start_logs`

Index **names** were not updated by the quest/run rename (`ALTER TABLE ... RENAME` doesn't rename dependent indexes) — several of the names above still say `instance`/`team` even though they now index `quest_id`/`run_code` columns on the renamed tables. Don't assume an index name reflects the current table/column name; check `internal/migrations/20260329000000_cascade_deletes.go` and `20260801000000_quest_run_renames.go` if in doubt.

## Enumerations

The database uses several enum types, most now implemented as strings rather than bare integers:

1. **RouteStrategy** (string: `ordered`, `free_roam`, `randomised`, `secret`) — controls how the next location/group is chosen for a player. Set per location group inside `GameStructure`.
2. **CompletionType** (string: `all`, `minimum`) — how a location group is considered complete. Replaces the old per-instance `CompletionMethod`.
3. **GameStatus** (int: `Scheduled`, `Active`, `Closed`) — computed on `Quest`, not stored.
4. **Provider** (string: `google`, or `""` for email/password) — a user's auth provider.

Dropped since the previous version of this document: `NavigationDisplayMode` and the old per-instance `CompletionMethod` (superseded by per-group settings in `GameStructure`), and the `Clue` table (removed entirely, no replacement).
