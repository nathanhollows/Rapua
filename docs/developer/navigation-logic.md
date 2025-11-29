---
title: "Navigation Logic Reference"
sidebar: true
order: 4
---

# Navigation Logic Reference

Quick reference for routing strategies, navigation modes, and completion settings.

---

## Routing Strategies

| ID | Name | Description |
|----|------|-------------|
| 0 | Random | Randomized locations with configurable max_next |
| 1 | FreeRoam | All locations available simultaneously |
| 2 | Ordered | Sequential access, one at a time |
| 3 | Secret | Never shown, accessible via direct access only |

## Navigation Display Modes

| ID | Name | Requires Coords? | Shows Names? |
|----|------|------------------|--------------|
| 0 | Map | Yes | No |
| 1 | MapAndNames | Yes | Yes |
| 2 | Names | No | Yes |
| 3 | Clues | No | No (deprecated) |
| 4 | Custom | No | No (uses blocks) |

## Completion Types

- **all**: All locations must be completed (forces auto-advance)
- **minimum**: N locations required (allows partial completion)

---

## Decision Tables

### Route Strategy Behavior

| Strategy | Visible? | Current Group? | Affects Progression? | Max Next? | Completion |
|----------|----------|----------------|---------------------|-----------|------------|
| Random   | ✓ | ✓ | ✓ | Required | Any |
| FreeRoam | ✓ | ✓ | ✓ | - | Any |
| Ordered  | ✓ | ✓ | ✓ | - | All (forced) |
| Secret   | ✗ | ✗ | ✗ | - | Disabled |

### UI Behavior

| Route | Completion Dropdown | Navigation Dropdown | Max Next Slider |
|-------|-------------------|-------------------|----------------|
| Random | Enabled | Enabled | Visible |
| FreeRoam | Enabled | Enabled | Hidden |
| Ordered | Disabled (All) | Enabled | Hidden |
| Secret | Disabled | Disabled | Hidden |

---

## Key Constraints

1. **Ordered** routing always uses **CompletionAll**
2. **Secret** groups are never the current group
3. **Secret** groups don't affect progression or game completion
4. **Secret** locations accessible when sibling or uncle to current group
5. **Random** routing requires MaxNext > 0
6. At least one non-secret group must exist

---

## Secret Location Access

Secret locations are accessible if they are **siblings of the current group or any ancestor** (walking up the tree to root).

**Accessible:**
- Siblings (same parent)
- Uncles (parent's siblings)
- Great-uncles (grandparent's siblings)
- Great-great-uncles... (any ancestor's siblings, recursively to root)

**NOT accessible:**
- Cousins (children of uncles - never goes DOWN the tree)
- Nested children (descendants of any group)

Example:
```
root[
  secret_root[loc9],           ← great-uncle (accessible)
  branch_a[
    secret_a[loc7, loc8],      ← uncle (accessible)
    branch_b[
      current[loc1, loc2],     ← you are here
      secret_b[loc3]           ← sibling (accessible)
    ],
    other_branch[
      secret_cousin[loc10]     ← cousin (NOT accessible)
    ]
  ]
]
```

If player in `current`:
- ✓ `loc3` (sibling)
- ✓ `loc7, loc8` (uncle - sibling of parent branch_b)
- ✓ `loc9` (great-uncle - sibling of grandparent branch_a)
- ✗ `loc10` (cousin - child of uncle, never accessible)

---

## Adding New Routing Strategies

### Checklist

1. **models/types.go** (5 updates required):
   - Add constant to const block (uses iota)
   - Add to `String()` method array
   - Add to `Description()` method array
   - Add to `GetRouteStrategies()` function
   - Add to `ParseRouteStrategy()` switch

2. **navigation/engine.go**:
   - Update `GetAvailableLocationIDs()` switch case
   - Update `GetFirstVisibleGroup()` if shouldn't be current group
   - Update `GetNextGroup()` if should be skipped in progression

3. **location_groups.templ**:
   - Add route option button with data attributes:
     - `data-routing="N"` (integer value)
     - `data-disable-completion="true/false"`
     - `data-disable-navigation="true/false"`
     - `data-show-max-next="true/false"`

4. **Update decision tables** in this doc

5. **Write tests** in `navigation/engine_test.go`:
   - Available locations behavior
   - Current group logic
   - Progression logic
   - UI state behavior

---

## Implementation Reference

- **Engine**: `navigation/engine.go`
- **Service**: `internal/services/navigation_service.go`
- **UI**: `internal/templates/admin/location_groups.templ`
- **Types**: `models/types.go`
- **Tests**: `navigation/engine_test.go`
