---
title: "Game Settings"
sidebar: true
order: 9
---

# Game Settings

## Quick Reference

| Setting | Options | What It Does |
|---------|---------|--------------|
| **Routing Strategy** | Randomised, Open Exploration, Guided Path, Secret | How players move between locations |
| **Navigation Display** | Map, Labelled Map, Location List, Custom Clues | How locations appear to players |
| **Completion Type** | All, Minimum | Whether all or some locations must be completed |
| **Auto-Advance** | On/Off | Automatically move to next group when minimum met |
| **Show Team Count** | On/Off | Display how many teams are at each location |
| **Check Method** | Check-In Only, Check-In/Out | How players complete locations |

---

## Example Game Structures

### Murder Mystery: Choice vs. Information
**Structure:** Multiple location groups by area, each with low minimum

```
Group 1: Crime Scene Area (5 locations, minimum 2)
  - Routing: Open Exploration
  - Auto-Advance: OFF
  - Display: Custom Clues

Group 2: Witness District (4 locations, minimum 1)
  - Routing: Open Exploration
  - Auto-Advance: OFF

Secret Group: Detective's Archives (3 bonus locations)
  - Additional clues for thorough investigators
  - Accessible anytime via QR codes hidden in main locations
```

**Why it works:** Players choose speed vs. thoroughness. Rush ahead and miss clues, or explore everything and get full information. No auto-advance means players control when they're ready to move on.

### Campus Scavenger Hunt with Bonuses
**Structure:** Open exploration with optional secret challenges

```
Main Group: Campus Landmarks (12 locations, minimum 8)
  - Routing: Open Exploration
  - Display: Labelled Map
  - Team Count: ON

Secret Group: All Libraries (5 locations)
  - Bonus points for collectors
  - Doesn't affect game completion
  - Serendipitous discovery via GPS radius
```

**Why it works:** Complete freedom with optional challenges. Secret group adds depth for engaged players without blocking others from finishing.

### Large Event: Spread and Guide
**Structure:** Random start, then linear story

```
Group 1: Opening Locations (10 locations, max 3 shown)
  - Routing: Randomised
  - Display: Labelled Map
  - Team Count: ON

Group 2: Story Sequence (6 locations)
  - Routing: Guided Path
  - Display: Custom Clues
  - Completion: All

Secret Group: Easter Eggs (hidden throughout)
  - Extra narrative or humor
  - Not required
```

**Why it works:** Randomised routing spreads crowds at start, then everyone converges into narrative sequence. Secrets add replayability.

### Progressive Challenge Hunt
**Structure:** Each area unlocks the next with increasing difficulty

```
Group 1: Beginner Area (8 locations, minimum 5)
  - Routing: Open Exploration
  - Auto-Advance: ON

Group 2: Intermediate Area (6 locations, minimum 4)
  - Routing: Open Exploration
  - Auto-Advance: ON

Group 3: Expert Area (4 locations, all required)
  - Routing: Guided Path
  - Display: Custom Clues

Secret Group: Master Challenge (2 locations)
  - For experts seeking extra difficulty
```

**Why it works:** Progressive difficulty with auto-advance. Players move forward when ready. Secret group provides post-game challenge.

---

## Setting Details

### Routing Strategy
How players progress through your game.

**Randomised Route**
- Randomly assigns locations from available pool
- Good for spreading players across large areas
- Requires "Max Locations" setting (how many shown at once)

**Open Exploration**
- All locations visible simultaneously
- Players choose their own path
- Best for exploration-based experiences

**Guided Path**
- Players visit locations in specific order
- Shows one location at a time
- Forces "All" completion (can't skip locations)

**Secret**
- Hidden bonus locations
- Never shown to players
- Only accessible via QR code, link, or GPS
- Doesn't affect game progression

### Navigation Display
How location information appears to players.

**Map**
- Visual map with unlabeled markers
- Requires GPS coordinates for locations

**Labelled Map**
- Map with location names shown
- Requires GPS coordinates

**Location List**
- Text list of location names
- No map required

**Custom Clues**
- Block-based custom content
- Show hints, images, puzzles instead of names/maps
- Most flexible - you design what players see

### Completion Type
How many locations must be completed to advance.

**All Locations**
- Every location in the group must be completed
- Automatically advances when group is complete
- Required for Guided Path routing

**Minimum N Locations**
- Complete at least N locations to advance
- Allows skipping some locations
- Works with Auto-Advance setting

### Auto-Advance
When to move to the next location group.

**Enabled**
- Automatically advances when minimum completion met
- Players don't manually trigger next group

**Disabled**
- Players stay in current group until all locations done
- Even if minimum is met, they can complete extras

### Structure Groups
Organize locations into phases or chapters.

- Each group has its own routing/navigation/completion settings
- Players progress through groups in order
- Use groups for multi-phase games or different gameplay styles
- Secret groups can exist alongside regular groups

### Show Team Count
Display how many teams are at each location.

**Enabled**
- Shows "3 teams here" at each location
- Helps with crowd management
- Useful for collaborative or competitive games

**Disabled**
- Hides team presence
- Maintains mystery/immersion

### Check Method
How players mark locations as complete.

**Check-In Only**
- Scan QR code once to complete
- Simple, quick progression

**Check-In and Check-Out**
- Scan to arrive, scan again to leave
- Tracks time spent at location
- Prevents premature progression
