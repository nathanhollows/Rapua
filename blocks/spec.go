package blocks

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateSpec produces a complete quest YAML specification as a Markdown document
// with YAML frontmatter.  The output covers the full format — top-level fields,
// settings, stage structure, stops, and every registered block type — and is
// suitable for committing to docs or pasting into an AI prompt.
func GenerateSpec() string {
	var sb strings.Builder

	// ── Frontmatter ──────────────────────────────────────────────────────────
	sb.WriteString("---\n")
	sb.WriteString("title: \"Quest YAML Specification\"\n")
	sb.WriteString("sidebar: true\n")
	sb.WriteString("order: 13\n")
	sb.WriteString("---\n\n")

	// ── Title & intro ─────────────────────────────────────────────────────────
	sb.WriteString("# Quest YAML Specification\n\n")
	sb.WriteString("This document is the authoritative reference for the Rapua quest YAML format.\n")
	sb.WriteString("It is auto-generated — do not edit by hand.\n")
	sb.WriteString("To regenerate after changing block definitions:\n\n")
	sb.WriteString("```\ngo test ./blocks/... -run TestBlockSpecGolden -update\n```\n\n")

	// ── Annotated example ────────────────────────────────────────────────────
	sb.WriteString("## Annotated Example\n\n")
	sb.WriteString("```yaml\n")
	sb.WriteString(annotatedExample)
	sb.WriteString("```\n\n")

	// ── Top-level fields ──────────────────────────────────────────────────────
	sb.WriteString("## Top-Level Fields\n\n")
	sb.WriteString("| Field | Type | Required | Description |\n")
	sb.WriteString("|-------|------|----------|-------------|\n")
	sb.WriteString("| `version` | int | yes | Format version. Must be `1`. |\n")
	sb.WriteString("| `id` | string | no | Quest UUID. Present in exported YAML; required when re-importing to update an existing quest (round-trip). Omit when creating a new quest from hand-written YAML. |\n")
	sb.WriteString("| `name` | string | yes | Quest display name. |\n")
	sb.WriteString("| `settings` | object | no | Game behaviour flags (see [Settings](#settings)). |\n")
	sb.WriteString("| `structure` | object | no | Stage grouping and routing config (see [Structure](#structure)). |\n")
	sb.WriteString("| `stops` | list | no | Stop definitions (see [Stops](#stops)). |\n")
	sb.WriteString("| `start` | list | no | Blocks shown on the pre-game start page. Same block syntax as stop content. |\n")
	sb.WriteString("| `finish` | list | no | Blocks shown on the post-game finish page. Same block syntax as stop content. |\n")
	sb.WriteString("\n")

	// ── Settings ──────────────────────────────────────────────────────────────
	sb.WriteString("## Settings\n\n")
	sb.WriteString("All fields are optional and default to `false`.\n\n")
	sb.WriteString("| Field | Type | Description |\n")
	sb.WriteString("|-------|------|-------------|\n")
	sb.WriteString("| `must_check_out` | bool | Players must check out of each stop before moving on. |\n")
	sb.WriteString("| `show_team_count` | bool | Show how many teams are currently at each stop. |\n")
	sb.WriteString("| `enable_points` | bool | Enable the points system. |\n")
	sb.WriteString("| `enable_bonus_points` | bool | Allow bonus points to be awarded. |\n")
	sb.WriteString("| `show_leaderboard` | bool | Display the leaderboard to players during the quest. |\n")
	sb.WriteString("\n")

	// ── Structure ─────────────────────────────────────────────────────────────
	sb.WriteString("## Structure\n\n")
	sb.WriteString("Stages group stops and control how players move through them.\n")
	sb.WriteString("Stages can be nested. Stops not referenced in any stage appear at the root level (shown as \"Unassigned\" on export).\n\n")
	sb.WriteString("### Stage Fields\n\n")
	sb.WriteString("| Field | Type | Required | Description |\n")
	sb.WriteString("|-------|------|----------|-------------|\n")
	sb.WriteString("| `name` | string | yes | Stage display name. The reserved value `\"Unassigned\"` places stops at the root level. |\n")
	sb.WriteString("| `color` | string | no | UI accent colour. Defaults to `\"primary\"`. |\n")
	sb.WriteString("| `routing` | enum | no | How stops are ordered for players (see below). Defaults to `\"free_roam\"`. |\n")
	sb.WriteString("| `navigation` | enum | no | Navigation UI shown to players (see below). Defaults to `\"map\"`. |\n")
	sb.WriteString("| `completion` | enum | no | What counts as completing this stage. Defaults to `\"all\"`. |\n")
	sb.WriteString("| `minimum_required` | int | no | Number of stops required when `completion: minimum`. |\n")
	sb.WriteString("| `max_next` | int | no | Maximum stops a player may visit next (0 = unlimited). |\n")
	sb.WriteString("| `auto_advance` | bool | no | Automatically advance to the next stage when this one is complete. |\n")
	sb.WriteString("| `stops` | list\\<string\\> | no | Slugs of stops that belong to this stage. |\n")
	sb.WriteString("| `stages` | list\\<Stage\\> | no | Nested sub-stages. |\n")
	sb.WriteString("\n")

	sb.WriteString("### Routing Values\n\n")
	sb.WriteString("| Value | Description |\n")
	sb.WriteString("|-------|-------------|\n")
	sb.WriteString("| `free_roam` | Players visit stops in any order. |\n")
	sb.WriteString("| `ordered` | Players must visit stops in the listed order. |\n")
	sb.WriteString("| `randomised` | Stop order is shuffled per team. |\n")
	sb.WriteString("| `secret` | Stop locations are hidden until the player is nearby. |\n")
	sb.WriteString("\n")

	sb.WriteString("### Navigation Values\n\n")
	sb.WriteString("| Value | Description |\n")
	sb.WriteString("|-------|-------------|\n")
	sb.WriteString("| `map` | Interactive map showing stop locations. |\n")
	sb.WriteString("| `labelled_map` | Map with stop names shown. |\n")
	sb.WriteString("| `location_list` | Plain list of stop names. |\n")
	sb.WriteString("| `custom` | Custom navigation content. |\n")
	sb.WriteString("| `tasks` | Task-list view. |\n")
	sb.WriteString("\n")

	sb.WriteString("### Completion Values\n\n")
	sb.WriteString("| Value | Description |\n")
	sb.WriteString("|-------|-------------|\n")
	sb.WriteString("| `all` | Players must complete every stop in the stage. |\n")
	sb.WriteString("| `minimum` | Players must complete at least `minimum_required` stops. |\n")
	sb.WriteString("\n")

	// ── Stops ─────────────────────────────────────────────────────────────────
	sb.WriteString("## Stops\n\n")
	sb.WriteString("| Field | Type | Required | Description |\n")
	sb.WriteString("|-------|------|----------|-------------|\n")
	sb.WriteString("| `id` | string | no | Stop UUID. Present in exported YAML; used for round-trip matching on re-import. Omit in hand-written YAML. |\n")
	sb.WriteString("| `slug` | string | yes | URL-safe identifier, unique within the quest (e.g. `old-govt-buildings`). Referenced in `structure.stages[].stops`. |\n")
	sb.WriteString("| `name` | string | yes | Stop display name shown to players. |\n")
	sb.WriteString("| `points` | int | no | Points awarded for checking in at this stop. |\n")
	sb.WriteString("| `marker` | object | no | Map coordinates for this stop (see below). |\n")
	sb.WriteString("| `content` | list\\<Block\\> | no | Blocks shown in the main content area of the stop. |\n")
	sb.WriteString("| `clues` | list\\<Block\\> | no | Clue blocks available at this stop. |\n")
	sb.WriteString("| `tasks` | list\\<Block\\> | no | Task blocks associated with this stop. |\n")
	sb.WriteString("| `checkpoint` | list\\<Block\\> | no | Blocks that gate stop completion (e.g. password, pincode). |\n")
	sb.WriteString("\n")

	sb.WriteString("### Marker Fields\n\n")
	sb.WriteString("| Field | Type | Required | Description |\n")
	sb.WriteString("|-------|------|----------|-------------|\n")
	sb.WriteString("| `lat` | float | yes | Latitude in decimal degrees. |\n")
	sb.WriteString("| `lng` | float | yes | Longitude in decimal degrees. |\n")
	sb.WriteString("\n")

	// ── Blocks intro ──────────────────────────────────────────────────────────
	sb.WriteString("## Blocks\n\n")
	sb.WriteString("Every block must have a `type` field. Two fields are common to all block types:\n\n")
	sb.WriteString("| Field | Type | Required | Description |\n")
	sb.WriteString("|-------|------|----------|-------------|\n")
	sb.WriteString("| `type` | string | yes | Block type identifier (see catalog below). |\n")
	sb.WriteString("| `id` | string | no | Block UUID. Present in exported YAML; enables round-trip updates that preserve player state (quiz answers, password completions, etc.). Omit in hand-written YAML. |\n")
	sb.WriteString("| `points` | int | no | Points awarded when a player completes this block. |\n")
	sb.WriteString("\n")

	sb.WriteString("### Block Contexts\n\n")
	sb.WriteString("Not every block type is valid in every context. The catalog below lists allowed contexts for each type.\n\n")
	sb.WriteString("| Context key | Used in |\n")
	sb.WriteString("|-------------|----------|\n")
	sb.WriteString("| `location_content` | `stops[].content` |\n")
	sb.WriteString("| `location_clues` | `stops[].clues` |\n")
	sb.WriteString("| `task` | `stops[].tasks` |\n")
	sb.WriteString("| `checkpoint` | `stops[].checkpoint` |\n")
	sb.WriteString("| `start` | top-level `start` |\n")
	sb.WriteString("| `finish` | top-level `finish` |\n")
	sb.WriteString("\n")

	// ── Block catalog ─────────────────────────────────────────────────────────
	sb.WriteString("## Block Catalog\n\n")

	types := make([]string, 0, len(blockRegistry))
	for t := range blockRegistry {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, blockType := range types {
		registration := blockRegistry[blockType]
		if registration == nil {
			continue
		}
		block := registration.Instance

		sb.WriteString(fmt.Sprintf("### %s\n\n", block.GetName()))
		sb.WriteString(fmt.Sprintf("**type:** `%s`  \n", blockType))
		sb.WriteString(fmt.Sprintf("**description:** %s\n\n", block.GetDescription()))

		ctxNames := make([]string, len(registration.SupportedContexts))
		for i, ctx := range registration.SupportedContexts {
			ctxNames[i] = fmt.Sprintf("`%s`", string(ctx))
		}
		sb.WriteString(fmt.Sprintf("**Allowed in:** %s\n\n", strings.Join(ctxNames, ", ")))

		spec := GetFieldSpec(blockType)
		if len(spec) == 0 {
			sb.WriteString("No additional fields beyond `type`, `id`, and `points`.\n\n")
		} else {
			sb.WriteString("**Fields:**\n\n")
			writeFieldTable(&sb, spec, 0)
			sb.WriteString("\n")
		}

		sb.WriteString("**Example:**\n\n")
		sb.WriteString("```yaml\n")
		sb.WriteString(fmt.Sprintf("- type: %s\n", blockType))
		writeYAMLExample(&sb, spec, 1)
		sb.WriteString("```\n\n")
	}

	return sb.String()
}

// writeFieldTable renders field definitions as a Markdown table, recursing into
// nested children with indented rows.
func writeFieldTable(sb *strings.Builder, fields []FieldDef, depth int) {
	if depth == 0 {
		sb.WriteString("| Field | Type | Required | Description |\n")
		sb.WriteString("|-------|------|----------|-------------|\n")
	}
	prefix := strings.Repeat("&nbsp;&nbsp;&nbsp;&nbsp;", depth)
	for _, f := range fields {
		name := f.Name
		if name == "" {
			name = "_(item)_"
		}
		req := ""
		if f.Required {
			req = "yes"
		}
		typeName := fieldTypeName(f.Type)
		desc := f.Desc
		if len(f.Enum) > 0 {
			quoted := make([]string, len(f.Enum))
			for i, v := range f.Enum {
				quoted[i] = fmt.Sprintf("`%s`", v)
			}
			if desc != "" {
				desc += " "
			}
			desc += "One of: " + strings.Join(quoted, ", ") + "."
		}
		sb.WriteString(fmt.Sprintf("| %s`%s` | %s | %s | %s |\n", prefix, name, typeName, req, desc))
		if len(f.Children) > 0 {
			writeFieldTable(sb, f.Children, depth+1)
		}
	}
}

// writeYAMLExample writes a minimal YAML snippet showing required fields
// (and the first enum value where applicable).
func writeYAMLExample(sb *strings.Builder, fields []FieldDef, indentLevel int) {
	indent := strings.Repeat("  ", indentLevel)
	for _, f := range fields {
		if f.Name == "" {
			continue
		}
		example := fieldExampleValue(f)
		sb.WriteString(fmt.Sprintf("%s%s: %s\n", indent, f.Name, example))
	}
}

// fieldExampleValue returns a representative YAML value for a field.
func fieldExampleValue(f FieldDef) string {
	switch f.Type {
	case FieldBool:
		return "false"
	case FieldInt:
		return "0"
	case FieldFloat:
		return "0.0"
	case FieldEnum:
		if len(f.Enum) > 0 {
			return f.Enum[0]
		}
		return `""`
	case FieldList:
		if len(f.Children) > 0 && f.Children[0].Name == "" {
			return `["item 1", "item 2"]`
		}
		return `[{...}]`
	case FieldMarkdown, FieldString:
		return `""`
	default:
		return `""`
	}
}

// fieldTypeName returns a human-readable name for a FieldType.
func fieldTypeName(t FieldType) string {
	switch t {
	case FieldString:
		return "string"
	case FieldBool:
		return "bool"
	case FieldInt:
		return "int"
	case FieldFloat:
		return "float"
	case FieldMarkdown:
		return "markdown"
	case FieldList:
		return "list"
	case FieldEnum:
		return "enum"
	default:
		return "unknown"
	}
}

// annotatedExample is the full annotated YAML example embedded in the spec.
const annotatedExample = `version: 1
id: "550e8400-e29b-41d4-a716-446655440000"  # omit when creating; required for round-trip updates
name: "My Quest"

settings:
  must_check_out: false       # players must check out before moving on
  show_team_count: false      # show how many teams are at each stop
  enable_points: true         # enable the points system
  enable_bonus_points: false  # allow bonus points
  show_leaderboard: true      # show leaderboard to players

structure:
  stages:
    - name: "Stage One"
      color: "primary"
      routing: "free_roam"    # free_roam | ordered | randomised | secret
      navigation: "map"       # map | labelled_map | location_list | custom | tasks
      completion: "all"       # all | minimum
      minimum_required: 0     # only used when completion: minimum
      auto_advance: false
      stops:
        - "old-govt-buildings"
        - "museum"
    - name: "Unassigned"      # reserved: stops placed at root level (not in a named stage)
      stops:
        - "bonus-stop"

stops:
  - id: "6ba7b810-9dad-11d1-80b4-00c04fd430c8"  # omit in hand-written YAML
    slug: "old-govt-buildings"
    name: "Old Government Buildings"
    points: 10
    marker:
      lat: -41.2784
      lng: 174.7767
    content:
      - type: text
        content: "Welcome to the oldest wooden government building in the world."
      - type: quiz
        id: "block-uuid"       # omit in hand-written YAML; preserves player state on re-import
        points: 5
        question: "What year was this building completed?"
        options:
          - text: "1876"
            correct: true
          - text: "1901"
            correct: false
    clues:
      - type: clue
        clue: "Look for the date carved above the main entrance."
        description: "Need a hint?"
    tasks:
      - type: task
        task: "Photograph the building's facade."
        link_through: true
    checkpoint:
      - type: password
        prompt: "Enter the secret code found at the entrance."
        answer: "rapua"
        unlocked_content: "Well done! Proceed to the next stop."

  - slug: "museum"
    name: "Te Papa Museum"
    content:
      - type: image
        url: "https://example.com/tepapa.jpg"
        caption: "Te Papa Tongarewa"

start:
  - type: team_name
    prompt: "Enter your team name to begin."
  - type: start_button
    active_text: "Start the Quest"

finish:
  - type: text
    content: "Congratulations on completing the quest!"
  - type: rating
    prompt: "How did you enjoy the quest?"
    max_rating: 5
`
