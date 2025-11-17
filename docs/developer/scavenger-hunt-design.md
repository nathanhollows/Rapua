---
title: "Scavenger Hunt Mode - Design Document"
sidebar: true
order: 20
---

# Scavenger Hunt Mode - Design Document

This document outlines the design for implementing Scavenger Hunt mode, a new gameplay strategy where players see all locations simultaneously and mark them as found.

## Overview

Unlike existing route strategies that filter available locations, Scavenger Hunt mode provides visibility into the entire hunt. Players can see all locations at once, track their progress, and complete them in any order.

---

## 1. RouteStrategyScavengerHunt

### Core Behavior

A single strategy that returns **all locations** with completion metadata attached:

```go
const (
    RouteStrategyRandom RouteStrategy = iota   // 0
    RouteStrategyFreeRoam                      // 1
    RouteStrategyOrdered                       // 2
    RouteStrategyScavengerHunt                 // 3 - NEW
)
```

**Key Differences from FreeRoam**:
- Navigation package returns uncompleted locations only
- Navigation service returns completed and uncompleted locations
- UI renders completion indicators

### Location Ordering

- Completed locations stay in their original position
- Maintains spatial/logical grouping intended by game designer

### Completion Logic

Respects existing `CompletionType` for group advancement:
- `CompletionAll`: Must find every location to advance
- `CompletionMinimum`: Must find at least N locations

### No Additional Configuration

The strategy itself requires no extra fields. Standard behaviors:
- Completion count always visible
- Points displayed per location
- Completion timestamps tracked

---

## 2. Navigation Display Modes

Two new display modes specific to scavenger hunts:

```go
const (
    NavigationDisplayMap NavigationDisplayMode = iota
    NavigationDisplayMapAndNames
    NavigationDisplayNames
    NavigationDisplayClues
    NavigationDisplayCustom
    NavigationDisplayScavList       // NEW
    NavigationDisplayScavPhotoGrid  // NEW
)
```

### NavigationDisplayScavList

**Purpose**: Vertical scrollable list of all locations

**Standard Features** (always visible):
- Completion counter: "Found 3/10"
- Per-location completion indicator (checkmark)
- Location name and optional hint text
- Points value per location
- Total points accumulated

**Visual States**:
- **Uncompleted**: Full opacity, tappable, navigates to location detail
- **Completed**: Checkmark icon, completion timestamp, muted appearance
- **In Validation**: Highlighted border (if player has scanned but not completed blocks)

**Ordering**: Preserves configured order; completed stay in place

### NavigationDisplayScavPhotoGrid

**Purpose**: Responsive image grid showing locations as tiles

**Standard Features**:
- Completion counter overlay
- Location photos from markers or custom images
- Completion overlay (checkmark stamp or grayscale effect)
- Location names below or overlaid on images

**Visual States**:
- **Uncompleted**: Full color image, tappable
- **Completed**: Grayscale or checkmark stamp overlay
- **Missing Image**: Placeholder with location name

**Grid Layout**: Responsive 2-3 columns based on screen width

---

## 3. Context-Per-Display Architecture

### New Block Contexts

Instead of overloading existing contexts, introduce display-specific contexts:

```go
const (
    ContextLocationContent      BlockContext = "location_content"
    ContextLocationClues        BlockContext = "location_clues"
    ContextCheckpoint           BlockContext = "checkpoint"
    ContextScavList             BlockContext = "scav_list"       // NEW
    ContextScavGrid             BlockContext = "scav_grid"       // NEW
)
```

### Rationale

- **Control block availability**: Only appropriate blocks appear in each display mode
- **Semantic clarity**: Context describes where the block renders
- **Future-proofing**: New scavenger display modes get their own contexts

### Block-Context Registration

| Block Type | Valid Contexts | Purpose |
|------------|----------------|---------|
| `scavenger_list` | `scav_list` | Renders vertical location list |
| `scavenger_photo_grid` | `scav_grid` | Renders image grid |

---

## 4. New Blocks

### ScavengerListBlock

**Type**: `scavenger_list`
**Context**: `ContextScavList`

**Rendered Output**:
- Vertical list of all locations
- Completion count header
- Per-item: name, hint (if available), points, completion status
- Tap behavior: navigate to location detail

**Data Requirements**:
```go
type ScavengerListData struct {
    // No configuration needed - behavior is standard
}
```

The block receives location list with completion metadata from navigation service.

### ScavengerPhotoGridBlock

**Type**: `scavenger_photo_grid`
**Context**: `ContextScavGrid`

**Rendered Output**:
- Responsive grid of location images
- Completion count header
- Per-tile: image, name overlay, completion indicator
- Tap behavior: navigate to location detail

**Data Requirements**:
```go
type ScavengerPhotoGridData struct {
    // No configuration needed - behavior is standard
}
```

---

## 5. User Experience Flow

### Standard Flow: Navigation → Scan → Validate → Return

**1. View Scavenger List/Grid**
```
Player opens navigation
→ Sees all 10 locations (3 completed, 7 remaining)
→ Completion counter: "Found 3/10"
→ Taps uncompleted location "Library"
```

**2. Navigate to Physical Location**
```
Player views location details
→ Sees hints, photos, or clues
→ Physically travels to Library
→ Finds QR code/marker
```

**3. Scan to Mark as Found**
```
Player scans QR code
→ CheckIn record created
→ Points awarded
→ Celebratory feedback (animation/sound)
```

**4. Optional Validation (if blocks exist)**
```
IF location has checkpoint blocks:
  → Player sees validation interface
  → Completes quiz/password/challenge
  → Additional points awarded
ELSE:
  → Simple confirmation screen
```

**5. Return to Hunt Overview**
```
Player returns to scavenger list/grid
→ Just-completed location now shows checkmark
→ Counter updates: "Found 4/10"
→ Momentum to continue hunt
```

### Alternate Flow: Mark Without Physical Scan

For less strict scavenger hunts:
```
Player taps location in list
→ Sees detail with "Mark as Found" button
→ Optional: validation block (prove knowledge)
→ Completion recorded without QR scan
```

This requires a group-level setting: `RequirePhysicalCheckIn: bool`

---

## 6. Data Flow

### PlayerNavigationView Enhancements

Current structure:
```go
type PlayerNavigationView struct {
    Settings           models.InstanceSettings
    CurrentGroup       *models.GameStructure
    NextLocations      []models.Location      // Available locations
    CompletedLocations []models.Location      // Already visited
    Blocks             []blocks.Block
    BlockStates        map[string]blocks.PlayerState
}
```

For scavenger hunts, `NextLocations` will contain ALL locations. The frontend needs completion metadata:

**Option A**: Enrich Location model with completion status
```go
type Location struct {
    // ... existing fields
    IsCompleted bool      // Computed at runtime
    CompletedAt time.Time // From CheckIn record
}
```

**Option B**: Return separate completion map
```go
type PlayerNavigationView struct {
    // ... existing fields
    CompletionStatus map[string]CompletionInfo // locationID → status
}

type CompletionInfo struct {
    IsCompleted bool
    CompletedAt time.Time
    PointsEarned int
}
```

Option A is simpler; Option B is cleaner separation.

### Navigation Engine Integration

```go
func GetAvailableLocationIDs(...) []string {
    switch group.Routing {
    case RouteStrategyScavengerHunt:
        // Return ALL location IDs, not filtered
        return group.LocationIDs
    case RouteStrategyOrdered:
        // Return single next
    case RouteStrategyRandom:
        // Return shuffled subset
    case RouteStrategyFreeRoam:
        // Return all unvisited
    }
}
```

The filtering happens in the UI layer, not the strategy.

---

## 7. Edge Cases and Mitigations

### Edge Case 1: Validation Block Not Completed
**Scenario**: Player scans QR but doesn't complete quiz
**Status**: Location is "found but not verified"
**Mitigation**:
- CheckIn exists with `BlocksCompleted = false`
- UI shows partial completion indicator
- Player must return to complete validation

### Edge Case 2: Large Hunt (50+ locations)
**Scenario**: List becomes overwhelming
**Mitigation**:
- Use nested groups (complete "Park Area" before seeing "Downtown")
- Progress milestones provide achievement feeling
- Filter: "Show remaining only" option

### Edge Case 3: Offline/Network Issues
**Scenario**: Player at remote location loses connectivity
**Mitigation**:
- Queue check-in for retry when online
- Show pending sync indicator
- Local state preservation

### Edge Case 4: Team Competition
**Scenario**: Multiple teams racing to complete same hunt
**Mitigation**:
- Current Team model supports this
- Add optional leaderboard block showing other teams' progress
- Real-time updates if needed

### Edge Case 5: Re-visiting Completed Locations
**Scenario**: Player wants to return to already-found location
**Mitigation**:
- Allow revisit (show location content again)
- Don't re-award points (CheckIn already exists)
- Clear visual that it's already completed

---

## 8. Future Extensions

### Team Mode Enhancements
- Show other teams' progress
- Collaborative completion (team members split locations)
- Leaderboard blocks

### Timed Hunts
- Countdown timer in progress block
- Time-based scoring bonuses
- Penalty for incomplete at deadline

### Riddles-First Mode
- Hide location identity until riddle solved
- Progressive reveal: solve riddle → see hint → find location
- Three-stage engagement

### Photo Verification
- Require photo upload as proof
- Geotagging validation
- Admin approval workflow

---

## 9. Implementation Order

1. ~**RouteStrategyScavengerHunt** - Add constant and strategy logic~
2. **New contexts** - Register `ContextScavList` and `ContextScavGrid`
3. **New blocks** - Implement `scavenger_list`, `scavenger_photo_grid`, `scavenger_progress`
4. **Display modes** - Extend `NavigationLocationNames` to render scavenger list
4. **Display modes** - Add `NavigationDisplayScavList` and `NavigationDisplayScavPhotoGrid`
5. ~**Navigation service** - Fetch and return completed locations~
6. **Templates** - Create block rendering templates
7. ~**Admin UI** - Allow selecting scavenger strategy and display modes~
7. **Admin UI** - Conditional setting restrictions based on scavenger mode
8. **Player UI** - Render scavenger navigation views

---

## 10. Design Principles

- **Minimal Configuration**: Standard behaviors are always enabled, no unnecessary toggles
- **Order Preservation**: Maintain game designer's intended location sequence
- **Context Control**: Display mode determines which blocks are valid
- **Celebratory Feedback**: Finding locations should feel rewarding
- **Clear State Transitions**: Player always knows what to do next
- **Future-Proof**: Architecture supports team modes, timers, and riddles without breaking changes
