---
title: "Database Schema"
sidebar: true
order: 3 
---

# Database Schema

This document outlines the database schema used in Rapua. The application uses SQLite with the Bun ORM to manage and query data.

## Schema Overview

```
┌───────────────┐        ┌────────────────┐        ┌─────────────┐
│    Instance   │        │    Location    │        │   Marker    │
├───────────────┤        ├────────────────┤        ├─────────────┤
│ id (PK)       │───┐    │ id (PK)        │        │ code (PK)   │
│ name          │   │    │ name           │        │ lat         │
│ user_id       │   │    │ instance_id    │────────│ lng         │
│ is_template   │   │    │ marker_id      │───┐    │ name        │
│ template_id   │   │    │ content_id     │   │    │ total_visits│
│ start_time    │   └────│ criteria       │   │    │ current_count
│ end_time      │        │ order          │   │    │ avg_duration│
└───────────────┘        └────────────────┘   │    └─────────────┘
        │                        │            │            
        │                        │            │            
        │                        │            │            
┌───────┴───────┐        ┌──────┴───────┐    │     ┌─────────────┐
│InstanceSettings│        │    Block     │    │     │   CheckIn   │
├───────────────┤        ├──────────────┤    │     ├─────────────┤
│ instance_id(PK)│        │ id (PK)      │    │     │ team_code   │
│ navigation_mode│        │ location_id  │    │     │ location_id │
│ navigation_meth│        │ type         │    │     │ time_in     │
│ max_next_loc   │        │ data         │    │     │ time_out    │
│ completion_meth│        │ ordering     │    │     │ must_check_out
│ show_team_count│        │ points       │    │     │ points      │
│ enable_points  │        │ validation_req│    │     └─────────────┘
└───────────────┘        └──────────────┘    │             │
        │                        │            │             │
        │                        │            │             │
┌───────┴───────┐        ┌──────┴───────┐    │     ┌───────┴─────┐
│     Team      │        │TeamBlockState │    │     │    Clue     │
├───────────────┤        ├──────────────┤    │     ├─────────────┤
│ code (PK)     │────────│ team_code    │    │     │ id (PK)     │
│ name          │        │ block_id     │    │     │ instance_id  │
│ instance_id   │        │ is_complete  │    │     │ location_id  │
│ has_started   │        │ points_awarded│    │     │ content     │
│ must_scan_out │────────┤ player_data  │    │     └─────────────┘
│ points        │        └──────────────┘    │             
└───────────────┘                           │             
        │                                   │             
        │                                   │             
┌───────┴───────┐                          │      ┌─────────────┐
│  Notification  │                          │      │    User     │
├───────────────┤                          │      ├─────────────┤
│ id (PK)       │                          │      │ id (PK)     │
│ content       │                          │      │ name        │
│ type          │                          │      │ email       │
│ team_code     │                          │      │ password    │
│ dismissed     │                          │      │ provider    │
└───────────────┘                          │      └─────────────┘
                                          │              │
                                          │              │
┌─────────────┐           ┌───────────────┴─┐    ┌──────┴──────┐
│ FacilitatorToken│        │   ShareLink     │    │   Upload    │
├─────────────┤           ├─────────────────┤    ├─────────────┤
│ token (PK)  │           │ id (PK)         │    │ id (PK)     │
│ instance_id │           │ template_id     │    │ user_id     │
│ created_by  │           │ created_by      │    │ filename    │
│ expires_at  │           │ name            │    │ size        │
└─────────────┘           └─────────────────┘    │ content_type│
                                                 └─────────────┘
```

## Tables

### Instance
The central table representing a game instance or template.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| name | string | Name of the instance |
| user_id | string | ID of the user who owns this instance |
| is_template | bool | Whether this instance is a reusable template |
| template_id | string | ID of the template this instance was created from (if any) |
| start_time | time | When the game instance is scheduled to start |
| end_time | time | When the game instance is scheduled to end |
| is_quick_start_dismissed | bool | Whether the quickstart guide has been dismissed |

### InstanceSettings
Settings that control how a game instance works.

| Field | Type | Description |
|-------|------|-------------|
| instance_id | string | Primary key, references instances.id |
| navigation_mode | int | How players navigate (linear, free choice, etc.) |
| navigation_method | int | Method of navigation (map, list, etc.) |
| max_next_locations | int | Maximum number of locations shown in the "next" view |
| completion_method | int | How location completion is determined |
| show_team_count | bool | Whether to show the number of teams at each location |
| enable_points | bool | Whether points are enabled for this game |
| enable_bonus_points | bool | Whether bonus points are enabled |
| show_leaderboard | bool | Whether to show the leaderboard to players |

### Location
A location or station in a game.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| name | string | Name of the location |
| instance_id | string | Foreign key to instances.id |
| marker_id | string | Foreign key to markers.code |
| content_id | string | Content identifier (historical) |
| criteria | string | Criteria for unlocking this location |
| order | int | Order in which this location appears |
| total_visits | int | Total number of team visits |
| current_count | int | Current number of teams at this location |
| avg_duration | float | Average time teams spend at this location |
| completion | int | How completion is determined for this location |
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
Content blocks that make up a location's interactive elements.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| location_id | string | Foreign key to locations.id |
| type | string | Block type (markdown, image, pincode, etc.) |
| data | json | Block-specific data in JSON format |
| ordering | int | Display order at the location |
| points | int | Points that can be awarded for this block |
| validation_required | bool | Whether validation is required to complete this block |

### TeamBlockState
Tracks the state of blocks for each team.

| Field | Type | Description |
|-------|------|-------------|
| team_code | string | Part of composite primary key, references teams.code |
| block_id | string | Part of composite primary key, references blocks.id |
| is_complete | bool | Whether the team has completed this block |
| points_awarded | int | Points awarded to the team for this block |
| player_data | json | Player-specific data for this block in JSON format |

### Team
A team of players participating in a game instance.

| Field | Type | Description |
|-------|------|-------------|
| code | string | Primary key, unique team code |
| name | string | Team name |
| instance_id | string | Foreign key to instances.id |
| has_started | bool | Whether the team has started the game |
| must_scan_out | string | Marker code that the team must scan to check out (if any) |
| points | int | Total points earned by the team |

### CheckIn
Records when teams check in and out of locations.

| Field | Type | Description |
|-------|------|-------------|
| team_code | string | Part of composite primary key, references teams.code |
| location_id | string | Part of composite primary key, references locations.id |
| instance_id | string | Foreign key to instances.id |
| time_in | time | When the team checked in |
| time_out | time | When the team checked out |
| must_check_out | bool | Whether check-out is required |
| points | int | Points awarded for this check-in |
| blocks_completed | bool | Whether all blocks at this location have been completed |

### Clue
Hints or clues about locations.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| instance_id | string | Foreign key to instances.id |
| location_id | string | Foreign key to locations.id |
| content | string | The clue content |

### Notification
Messages sent to teams during gameplay.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| content | string | Notification content |
| type | string | Notification type |
| team_code | string | Foreign key to teams.code |
| dismissed | bool | Whether the notification has been dismissed |

### User
User accounts for game administrators.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| name | string | User's name |
| email | string | User's email (unique) |
| email_verified | bool | Whether the email has been verified |
| email_token | string | Token for email verification |
| email_token_expiry | time | When the email token expires |
| password | string | Hashed password |
| provider | string | Authentication provider (if using OAuth) |
| current_instance_id | string | ID of the currently active instance |

### FacilitatorToken
Tokens that allow facilitators to access game instances.

| Field | Type | Description |
|-------|------|-------------|
| token | string | Primary key, unique token |
| instance_id | string | Foreign key to instances.id |
| created_by | string | User ID of the creator |
| expires_at | time | When the token expires |

### Upload
Uploaded files (images, etc.)

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| user_id | string | Foreign key to users.id |
| filename | string | Original filename |
| size | int | File size in bytes |
| content_type | string | MIME type of the file |

### ShareLink
Links that allow sharing templates.

| Field | Type | Description |
|-------|------|-------------|
| id | string | Primary key, unique identifier |
| template_id | string | Foreign key to instances.id (where is_template=true) |
| created_by | string | User ID of the creator |
| name | string | Display name for the share link |

## Key Relationships

1. **Instance to Locations**: One-to-many. Each game instance has multiple locations.

2. **Location to Blocks**: One-to-many. Each location has multiple content blocks.

3. **Instance to Teams**: One-to-many. Each game instance has multiple teams.

4. **Team to CheckIns**: One-to-many. Teams can check in to multiple locations.

5. **Location to Marker**: Many-to-one. Multiple locations can use the same marker.

6. **Team to TeamBlockState**: One-to-many. Teams have state for each block they interact with.

7. **User to Instances**: One-to-many. Users can create multiple game instances.

8. **Instance to Template**: Many-to-one. Many instances can be created from one template.

9. **Template to ShareLinks**: One-to-many. A template can have multiple share links.

## Database Indexes

The schema maintains indexes on all primary keys and foreign key relationships to ensure quick lookups. Notable indexes include:

- `team_code` and `block_id` in TeamBlockState (composite primary key)
- `team_code` and `location_id` in CheckIn (composite primary key)
- `instance_id` in Location (for finding all locations in a game)
- `marker_id` in Location (for finding locations by marker code)
- `location_id` in Block (for finding all blocks at a location)

## Enumerations

The database uses several enum types implemented as integers:

1. **NavigationMode**:
   - Controls how players navigate between locations

2. **NavigationMethod**:
   - Determines what method players use to find locations

3. **CompletionMethod**:
   - How location completion is determined (all blocks, specific blocks, etc.)

4. **GameStatus**:
   - Represents game states: Closed, Scheduled, Active
