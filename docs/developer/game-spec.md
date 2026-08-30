---
title: "Game Spec"
sidebar: true
order: 21
---

# Game Spec

> Generated from code. Regenerate with: `rapua genspec`
>
> Machine-readable: `GET /api/v8/spec`

## Authoring constraints

These rules are enforced by the linter (`POST /api/v8/lint`). Errors block import; warnings should be fixed.

**Structure**
- Every objective must be inside a group. An objective placed directly under `structure.children` is never shown to players. Wrap it in a group. *(`ROOT_OBJECTIVE_HIDDEN`)*
- Groups must have at least one child. An empty group produces a warning. *(`EMPTY_GROUP`)*
- Objective slugs must be unique across the entire game, including across groups. *(`SLUG_DUPLICATE`)*

**Import modes**
- **Create-import** (`POST /admin/quests/import`): omit `id` on objectives and blocks: new UUIDs are generated.
- **Update-import** (`POST /admin/quests/{id}/import`): include `id` to reconcile with existing records. Matched blocks preserve player state (`RunBlockState`). Objectives absent from the document are deleted.
- Group `id` is preserved on update-import to avoid orphaning team progress records (`SkippedGroupIDs`).

**Blocks**
- Every block must have a `type` field matching a registered block type.
- A block may only appear in contexts listed in its spec. *(`INVALID_CONTEXT`)*
- Block `id` values must be unique across the document. *(`BLOCK_ID_DUPLICATE`)*
- Block `points` are ignored unless `settings.enable_points` is true. *(`POINTS_DISABLED` warning)* An objective has no `points` field of its own: its total point value is the sum of its blocks' points.

**Start page**
- A start page with blocks but no `start_button` block will not let players join. *(`NO_START_BUTTON` warning)*

**Completion**
- `minimum_required` is only valid when `completion` is `"minimum"`; it must be a positive integer. *(`MINIMUM_REQUIRED_MISMATCH` / `MINIMUM_REQUIRED_MISSING`)*

**Conditional visibility (`when` / `sets`)**
- Every variable referenced in a `when` condition must be defined in a block `sets` or the built-in variable list. *(`UNDEFINED_VAR`)*
- No two `sets` declarations across the whole game may write the same variable name. *(`DUPLICATE_SETS_VAR`)*
- `sets` variable names must not shadow built-in variable names. *(`SHADOWED_VAR`)*
- `sets` is a list of variable names set to `"true"` when the block completes.
- `op` in a condition must be a valid operator from `enums.condition_ops`. *(`INVALID_CONDITION_OP`)*
- Every condition must have a `var` field; `value` is required when `op` is present. *(`INVALID_CONDITION`)*
- `sets` on a content block (text, alert, image, etc.) is ignored. *(`SETS_ON_CONTENT_BLOCK` warning)*
- A `sets` variable that is never referenced in any `when` clause produces a warning. *(`UNUSED_SETS_VAR` warning)*

## Full spec

```json
{
  "version": "v8",
  "document": {
    "description": "Top-level v8 game document.",
    "fields": [
      {
        "name": "rapua",
        "type": "string",
        "description": "Format version. Must be \"v8\".",
        "required": true
      },
      {
        "name": "id",
        "type": "string",
        "description": "Instance UUID. Present on export; omit on create-import to generate a new UUID."
      },
      {
        "name": "name",
        "type": "string",
        "description": "Game name.",
        "required": true
      },
      {
        "name": "settings",
        "type": "object",
        "description": "Game-wide settings.",
        "required": true,
        "fields": [
          {
            "name": "show_team_count",
            "type": "bool",
            "description": "Show how many teams are at each objective."
          },
          {
            "name": "enable_points",
            "type": "bool",
            "description": "Enable the points system."
          },
          {
            "name": "show_leaderboard",
            "type": "bool",
            "description": "Show the leaderboard to players."
          }
        ]
      },
      {
        "name": "start",
        "type": "list",
        "description": "Blocks shown on the start page. Always present, even if empty.",
        "required": true
      },
      {
        "name": "finish",
        "type": "list",
        "description": "Blocks shown on the finish page. Always present, even if empty.",
        "required": true
      },
      {
        "name": "structure",
        "type": "object",
        "description": "Root group defining routing and the objective tree.",
        "required": true,
        "fields": [
          {
            "name": "color",
            "type": "string",
            "description": "Display colour for this group (e.g. \"primary\", \"secondary\"). Omit or empty for the root group."
          },
          {
            "name": "routing",
            "type": "enum",
            "description": "How players are routed through objectives. See enums.routing.",
            "required": true
          },
          {
            "name": "completion",
            "type": "enum",
            "description": "When the group is considered complete. See enums.completion.",
            "required": true
          },
          {
            "name": "minimum_required",
            "type": "int",
            "description": "Number of objectives required when completion is \"minimum\"."
          },
          {
            "name": "when",
            "type": "object",
            "description": "Visibility conditions. Element is hidden when conditions are not met. Absent means always visible.",
            "fields": [
              {
                "name": "all_of",
                "type": "list",
                "description": "ALL conditions must be true (AND). Each item is a condition object.",
                "items": {
                  "name": "",
                  "type": "object",
                  "description": "A single condition. var is required; op+value are optional comparisons; not negates the result.",
                  "fields": [
                    {
                      "name": "var",
                      "type": "string",
                      "description": "Variable to check. Built-in: player.points, run.started_at, objective.\u003cslug\u003e, game.team_count. Creator-defined via block sets.",
                      "required": true
                    },
                    {
                      "name": "op",
                      "type": "enum",
                      "description": "Comparison operator. Omit for a bare truthy check. See enums.condition_ops.",
                      "enum": [
                        "eq",
                        "neq",
                        "gt",
                        "lt",
                        "gte",
                        "lte",
                        "in",
                        "not_in"
                      ]
                    },
                    {
                      "name": "value",
                      "type": "any",
                      "description": "Value to compare against. String, int, bool, or array (for in/not_in). Required when op is present."
                    },
                    {
                      "name": "not",
                      "type": "bool",
                      "description": "Negate the result of this condition."
                    }
                  ]
                }
              },
              {
                "name": "any_of",
                "type": "list",
                "description": "At least one condition must be true (OR). Each item is a condition object.",
                "items": {
                  "name": "",
                  "type": "object",
                  "description": "A single condition. var is required; op+value are optional comparisons; not negates the result.",
                  "fields": [
                    {
                      "name": "var",
                      "type": "string",
                      "description": "Variable to check. Built-in: player.points, run.started_at, objective.\u003cslug\u003e, game.team_count. Creator-defined via block sets.",
                      "required": true
                    },
                    {
                      "name": "op",
                      "type": "enum",
                      "description": "Comparison operator. Omit for a bare truthy check. See enums.condition_ops.",
                      "enum": [
                        "eq",
                        "neq",
                        "gt",
                        "lt",
                        "gte",
                        "lte",
                        "in",
                        "not_in"
                      ]
                    },
                    {
                      "name": "value",
                      "type": "any",
                      "description": "Value to compare against. String, int, bool, or array (for in/not_in). Required when op is present."
                    },
                    {
                      "name": "not",
                      "type": "bool",
                      "description": "Negate the result of this condition."
                    }
                  ]
                }
              }
            ]
          },
          {
            "name": "children",
            "type": "list",
            "description": "Ordered list of objective or group children.",
            "required": true,
            "items": {
              "name": "",
              "type": "object",
              "description": "Tagged union: set exactly one of \"objective\" or \"group\".",
              "fields": [
                {
                  "name": "objective",
                  "type": "object",
                  "description": "A single game objective. See objective schema."
                },
                {
                  "name": "group",
                  "type": "object",
                  "description": "A named sub-group with its own routing and completion settings."
                }
              ]
            }
          }
        ]
      },
      {
        "name": "objective",
        "type": "object",
        "description": "Schema for objective objects within structure.children. Has no points field of its own; its total point value is the sum of its blocks' points.",
        "fields": [
          {
            "name": "id",
            "type": "string",
            "description": "Objective UUID. Present on export; omit on create-import to generate a new UUID."
          },
          {
            "name": "slug",
            "type": "string",
            "description": "Short alphanumeric code referenced by objective.\u003cslug\u003e when clauses. Must be unique within the game.",
            "required": true
          },
          {
            "name": "title",
            "type": "string",
            "description": "Display title shown to players.",
            "required": true
          },
          {
            "name": "when",
            "type": "object",
            "description": "Visibility conditions. Element is hidden when conditions are not met. Absent means always visible.",
            "fields": [
              {
                "name": "all_of",
                "type": "list",
                "description": "ALL conditions must be true (AND). Each item is a condition object.",
                "items": {
                  "name": "",
                  "type": "object",
                  "description": "A single condition. var is required; op+value are optional comparisons; not negates the result.",
                  "fields": [
                    {
                      "name": "var",
                      "type": "string",
                      "description": "Variable to check. Built-in: player.points, run.started_at, objective.\u003cslug\u003e, game.team_count. Creator-defined via block sets.",
                      "required": true
                    },
                    {
                      "name": "op",
                      "type": "enum",
                      "description": "Comparison operator. Omit for a bare truthy check. See enums.condition_ops.",
                      "enum": [
                        "eq",
                        "neq",
                        "gt",
                        "lt",
                        "gte",
                        "lte",
                        "in",
                        "not_in"
                      ]
                    },
                    {
                      "name": "value",
                      "type": "any",
                      "description": "Value to compare against. String, int, bool, or array (for in/not_in). Required when op is present."
                    },
                    {
                      "name": "not",
                      "type": "bool",
                      "description": "Negate the result of this condition."
                    }
                  ]
                }
              },
              {
                "name": "any_of",
                "type": "list",
                "description": "At least one condition must be true (OR). Each item is a condition object.",
                "items": {
                  "name": "",
                  "type": "object",
                  "description": "A single condition. var is required; op+value are optional comparisons; not negates the result.",
                  "fields": [
                    {
                      "name": "var",
                      "type": "string",
                      "description": "Variable to check. Built-in: player.points, run.started_at, objective.\u003cslug\u003e, game.team_count. Creator-defined via block sets.",
                      "required": true
                    },
                    {
                      "name": "op",
                      "type": "enum",
                      "description": "Comparison operator. Omit for a bare truthy check. See enums.condition_ops.",
                      "enum": [
                        "eq",
                        "neq",
                        "gt",
                        "lt",
                        "gte",
                        "lte",
                        "in",
                        "not_in"
                      ]
                    },
                    {
                      "name": "value",
                      "type": "any",
                      "description": "Value to compare against. String, int, bool, or array (for in/not_in). Required when op is present."
                    },
                    {
                      "name": "not",
                      "type": "bool",
                      "description": "Negate the result of this condition."
                    }
                  ]
                }
              }
            ]
          },
          {
            "name": "proof",
            "type": "object",
            "description": "Blocks and sets shown/fired while the objective is unproven. A non-empty proof must contain at least one interactive block, or it gates nothing.",
            "required": true,
            "fields": [
              {
                "name": "blocks",
                "type": "list",
                "description": "Blocks shown to players while this context is active."
              },
              {
                "name": "sets",
                "type": "object",
                "description": "Variables written when this block completes, as an object of {name: value}. Values may be strings, numbers, or booleans; all are stored as strings. Any other shape emits SETS_NOT_OBJECT. Only valid on interactive blocks — linter emits SETS_ON_CONTENT_BLOCK warning otherwise. Writing to the reserved \"objective.*\" namespace emits SETS_RESERVED_NAMESPACE — that prefix is owned by the runtime and set automatically when objectives complete.",
                "items": {
                  "name": "",
                  "type": "string"
                }
              }
            ]
          },
          {
            "name": "reveal",
            "type": "object",
            "description": "Blocks and sets shown/fired once proof completes.",
            "required": true,
            "fields": [
              {
                "name": "blocks",
                "type": "list",
                "description": "Blocks shown to players while this context is active."
              },
              {
                "name": "sets",
                "type": "object",
                "description": "Variables written when this block completes, as an object of {name: value}. Values may be strings, numbers, or booleans; all are stored as strings. Any other shape emits SETS_NOT_OBJECT. Only valid on interactive blocks — linter emits SETS_ON_CONTENT_BLOCK warning otherwise. Writing to the reserved \"objective.*\" namespace emits SETS_RESERVED_NAMESPACE — that prefix is owned by the runtime and set automatically when objectives complete.",
                "items": {
                  "name": "",
                  "type": "string"
                }
              }
            ]
          }
        ]
      }
    ]
  },
  "enums": {
    "routing": [
      {
        "value": "randomised",
        "label": "Randomised Route",
        "description": "The game will randomly select objectives for players to pursue. Good for large groups as it disperses players."
      },
      {
        "value": "free_roam",
        "label": "Open Exploration",
        "description": "Players can pursue objectives in any order. This mode shows all objectives and is good for exploration."
      },
      {
        "value": "ordered",
        "label": "Guided Path",
        "description": "Players must complete objectives in a specific order. Good for narrative experiences."
      },
      {
        "value": "secret",
        "label": "Secret",
        "description": "Objectives that may be accessed out of sequence. These objectives are never explicitly shown to players."
      }
    ],
    "completion": [
      {
        "value": "all",
        "label": "All Objectives",
        "description": "All objectives must be completed for the group to be considered done."
      },
      {
        "value": "minimum",
        "label": "Minimum Required",
        "description": "A minimum number of objectives must be completed for the group to be considered done."
      }
    ],
    "condition_ops": [
      {
        "value": "eq",
        "label": "Equal",
        "description": "var == value"
      },
      {
        "value": "neq",
        "label": "Not equal",
        "description": "var != value"
      },
      {
        "value": "gt",
        "label": "Greater than",
        "description": "var \u003e value (numeric)"
      },
      {
        "value": "lt",
        "label": "Less than",
        "description": "var \u003c value (numeric)"
      },
      {
        "value": "gte",
        "label": "Greater than or equal",
        "description": "var \u003e= value (numeric)"
      },
      {
        "value": "lte",
        "label": "Less than or equal",
        "description": "var \u003c= value (numeric)"
      },
      {
        "value": "in",
        "label": "In array",
        "description": "var is one of value (value must be an array)"
      },
      {
        "value": "not_in",
        "label": "Not in array",
        "description": "var is not in value (value must be an array)"
      }
    ]
  },
  "built_in_vars": [
    {
      "var": "player.points",
      "type": "int",
      "description": "Total points earned on this run. Evaluated live from the run's points."
    },
    {
      "var": "points",
      "type": "int",
      "description": "Pre-respine spelling of player.points. Still resolves; prefer player.points."
    },
    {
      "var": "run.started_at",
      "type": "timestamp",
      "description": "RFC3339 timestamp of when the run began. Empty until the players start."
    },
    {
      "var": "objective.\u003cslug\u003e",
      "type": "string",
      "description": "Resolves to \"done\" when the objective with the given slug is completed, empty string otherwise."
    },
    {
      "var": "game.team_count",
      "type": "int",
      "description": "Number of teams with HasStarted == true in this game instance."
    }
  ],
  "contexts": [
    {
      "value": "start",
      "description": "Blocks shown on the game start page (introductions, rules, team name, start button)."
    },
    {
      "value": "finish",
      "description": "Blocks shown on the game finish/end page."
    },
    {
      "value": "objective_proof",
      "description": "Blocks a player must complete to prove an objective. Once every block here completes, the reveal context is shown."
    },
    {
      "value": "objective_reveal",
      "description": "Blocks shown once an objective's proof context is complete."
    }
  ],
  "block_shared_fields": [
    {
      "name": "when",
      "type": "object",
      "description": "Visibility conditions. Element is hidden when conditions are not met. Absent means always visible.",
      "fields": [
        {
          "name": "all_of",
          "type": "list",
          "description": "ALL conditions must be true (AND). Each item is a condition object.",
          "items": {
            "name": "",
            "type": "object",
            "description": "A single condition. var is required; op+value are optional comparisons; not negates the result.",
            "fields": [
              {
                "name": "var",
                "type": "string",
                "description": "Variable to check. Built-in: player.points, run.started_at, objective.\u003cslug\u003e, game.team_count. Creator-defined via block sets.",
                "required": true
              },
              {
                "name": "op",
                "type": "enum",
                "description": "Comparison operator. Omit for a bare truthy check. See enums.condition_ops.",
                "enum": [
                  "eq",
                  "neq",
                  "gt",
                  "lt",
                  "gte",
                  "lte",
                  "in",
                  "not_in"
                ]
              },
              {
                "name": "value",
                "type": "any",
                "description": "Value to compare against. String, int, bool, or array (for in/not_in). Required when op is present."
              },
              {
                "name": "not",
                "type": "bool",
                "description": "Negate the result of this condition."
              }
            ]
          }
        },
        {
          "name": "any_of",
          "type": "list",
          "description": "At least one condition must be true (OR). Each item is a condition object.",
          "items": {
            "name": "",
            "type": "object",
            "description": "A single condition. var is required; op+value are optional comparisons; not negates the result.",
            "fields": [
              {
                "name": "var",
                "type": "string",
                "description": "Variable to check. Built-in: player.points, run.started_at, objective.\u003cslug\u003e, game.team_count. Creator-defined via block sets.",
                "required": true
              },
              {
                "name": "op",
                "type": "enum",
                "description": "Comparison operator. Omit for a bare truthy check. See enums.condition_ops.",
                "enum": [
                  "eq",
                  "neq",
                  "gt",
                  "lt",
                  "gte",
                  "lte",
                  "in",
                  "not_in"
                ]
              },
              {
                "name": "value",
                "type": "any",
                "description": "Value to compare against. String, int, bool, or array (for in/not_in). Required when op is present."
              },
              {
                "name": "not",
                "type": "bool",
                "description": "Negate the result of this condition."
              }
            ]
          }
        }
      ]
    },
    {
      "name": "sets",
      "type": "object",
      "description": "Variables written when this block completes, as an object of {name: value}. Values may be strings, numbers, or booleans; all are stored as strings. Any other shape emits SETS_NOT_OBJECT. Only valid on interactive blocks — linter emits SETS_ON_CONTENT_BLOCK warning otherwise. Writing to the reserved \"objective.*\" namespace emits SETS_RESERVED_NAMESPACE — that prefix is owned by the runtime and set automatically when objectives complete.",
      "items": {
        "name": "",
        "type": "string"
      }
    },
    {
      "name": "points",
      "type": "int",
      "description": "Points awarded to the player when this block completes. Negative for a block framed as a cost rather than a reward (e.g. paying points to reveal a clue). Ignored unless settings.enable_points is true. An objective's total point value is the sum of its blocks' points; it is not a field set on the objective itself."
    }
  ],
  "blocks": [
    {
      "type": "alert",
      "name": "Alert",
      "description": "Displays a styled alert box with a message.",
      "contexts": [
        "finish",
        "start",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "content",
          "type": "string",
          "description": "Alert message text",
          "required": true
        },
        {
          "name": "style",
          "type": "enum",
          "description": "Visual style of the alert",
          "enum": [
            "info",
            "success",
            "warning",
            "error"
          ]
        }
      ]
    },
    {
      "type": "broker",
      "name": "Information Broker",
      "description": "Players spend points to reveal progressively detailed information tiers. Points are spent via each tier's points_required, deducted from the team on purchase; the broker has no block-level points field.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "sets"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "Instructions shown to the player"
        },
        {
          "name": "default_info",
          "type": "markdown",
          "description": "Information shown for free (0 points)"
        },
        {
          "name": "tiers",
          "type": "list",
          "description": "Paid information tiers in ascending cost order",
          "items": {
            "name": "",
            "type": "object",
            "fields": [
              {
                "name": "points_required",
                "type": "int",
                "description": "Points cost for this tier",
                "required": true
              },
              {
                "name": "content",
                "type": "markdown",
                "description": "Information revealed at this tier",
                "required": true
              }
            ]
          }
        }
      ]
    },
    {
      "type": "button",
      "name": "Button",
      "description": "A clickable button that opens a URL.",
      "contexts": [
        "finish",
        "start",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "link",
          "type": "string",
          "description": "URL the button opens",
          "required": true
        },
        {
          "name": "text",
          "type": "string",
          "description": "Button label text",
          "required": true
        },
        {
          "name": "style",
          "type": "enum",
          "description": "Button visual style",
          "enum": [
            "primary",
            "secondary",
            "accent",
            "ghost"
          ]
        }
      ]
    },
    {
      "type": "checklist",
      "name": "Checklist",
      "description": "An interactive checklist players can tick off.",
      "contexts": [
        "start",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "content",
          "type": "markdown",
          "description": "Optional introductory text above the checklist"
        },
        {
          "name": "items",
          "type": "list",
          "description": "Checklist items",
          "required": true,
          "items": {
            "name": "",
            "type": "object",
            "fields": [
              {
                "name": "id",
                "type": "string",
                "description": "Unique item identifier"
              },
              {
                "name": "description",
                "type": "string",
                "description": "Item text",
                "required": true
              },
              {
                "name": "checked",
                "type": "bool",
                "description": "Default checked state"
              }
            ]
          }
        }
      ]
    },
    {
      "type": "choice",
      "name": "Choice",
      "description": "Presents labelled options; selecting one sets a boolean variable.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "Question or instruction shown above the choices",
          "required": true
        },
        {
          "name": "button_text",
          "type": "string",
          "description": "Label for the submit button (default: \"Confirm choice\")"
        },
        {
          "name": "multi_select",
          "type": "bool",
          "description": "Allow selecting multiple options (default: false — single choice)"
        },
        {
          "name": "options",
          "type": "list",
          "description": "Choices presented to the player",
          "required": true,
          "items": {
            "name": "",
            "type": "object",
            "fields": [
              {
                "name": "label",
                "type": "string",
                "description": "Display text for this choice",
                "required": true
              },
              {
                "name": "sets",
                "type": "string",
                "description": "Variable name set to \"true\" when this choice is selected",
                "required": true
              }
            ]
          }
        }
      ]
    },
    {
      "type": "clue",
      "name": "Clue",
      "description": "A clue revealed behind a button — players tap to reveal the hint.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "clue",
          "type": "markdown",
          "description": "Clue content revealed when the button is tapped",
          "required": true
        },
        {
          "name": "description",
          "type": "string",
          "description": "Description or label shown on the reveal button"
        },
        {
          "name": "button_label",
          "type": "string",
          "description": "Custom button label (defaults to 'Reveal clue')"
        }
      ]
    },
    {
      "type": "divider",
      "name": "Divider",
      "description": "A horizontal rule with an optional title label.",
      "contexts": [
        "finish",
        "start",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "title",
          "type": "string",
          "description": "Optional label shown on the divider"
        }
      ]
    },
    {
      "type": "free_text",
      "name": "Free Text",
      "description": "An ungraded text input for player reflections and free-form responses.",
      "contexts": [
        "finish",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "Question or instruction shown to the player"
        },
        {
          "name": "placeholder",
          "type": "string",
          "description": "Placeholder text for the input field"
        }
      ]
    },
    {
      "type": "game_status",
      "name": "Game Status Alert",
      "description": "Shows the current game status (scheduled, active, or closed) with optional messages.",
      "contexts": [
        "start"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "closed_message",
          "type": "string",
          "description": "Message shown when the game is closed"
        },
        {
          "name": "scheduled_message",
          "type": "string",
          "description": "Message shown when the game is scheduled"
        },
        {
          "name": "show_countdown",
          "type": "bool",
          "description": "Show a countdown timer when game is scheduled"
        }
      ]
    },
    {
      "type": "header",
      "name": "Header",
      "description": "A page header with an icon and title.",
      "contexts": [
        "start",
        "finish",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "icon",
          "type": "string",
          "description": "Icon name (Lucide icon)"
        },
        {
          "name": "title",
          "type": "string",
          "description": "Header title text",
          "required": true
        },
        {
          "name": "size",
          "type": "enum",
          "description": "Title font size",
          "enum": [
            "sm",
            "md",
            "lg",
            "xl",
            "2xl",
            "3xl"
          ]
        }
      ]
    },
    {
      "type": "image",
      "name": "Image",
      "description": "Displays an image with an optional caption and link.",
      "contexts": [
        "finish",
        "start",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "url",
          "type": "string",
          "description": "Image URL",
          "required": true
        },
        {
          "name": "caption",
          "type": "string",
          "description": "Caption displayed below the image"
        },
        {
          "name": "link",
          "type": "string",
          "description": "Optional URL to open when the image is clicked"
        },
        {
          "name": "full_width",
          "type": "bool",
          "description": "Render image edge-to-edge on mobile screens"
        }
      ]
    },
    {
      "type": "map",
      "name": "Map",
      "description": "Displays a Mapbox map centred on a specific location with a marker.",
      "contexts": [
        "finish",
        "start",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "latitude",
          "type": "float",
          "description": "Map centre latitude",
          "required": true
        },
        {
          "name": "longitude",
          "type": "float",
          "description": "Map centre longitude",
          "required": true
        },
        {
          "name": "zoom",
          "type": "float",
          "description": "Map zoom level (1–20, default 14)"
        },
        {
          "name": "caption",
          "type": "string",
          "description": "Optional caption displayed below the map"
        },
        {
          "name": "hide_marker",
          "type": "bool",
          "description": "When true, the map pin is hidden on the player view"
        }
      ]
    },
    {
      "type": "password",
      "name": "Password",
      "description": "Players must enter the correct text answer to proceed.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "Question or instruction shown to the player"
        },
        {
          "name": "answer",
          "type": "string",
          "description": "Correct answer (case-insensitive)",
          "required": true
        },
        {
          "name": "unlocked_content",
          "type": "markdown",
          "description": "Content shown after a correct answer"
        }
      ]
    },
    {
      "type": "photo",
      "name": "Photo Upload",
      "description": "Players upload one or more photos as their response.",
      "contexts": [
        "finish",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "Instructions shown to the player"
        },
        {
          "name": "max_images",
          "type": "int",
          "description": "Maximum number of photos the player can upload"
        }
      ]
    },
    {
      "type": "pincode",
      "name": "Pin Code",
      "description": "Players must enter a numeric PIN to proceed.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "Question or instruction shown to the player"
        },
        {
          "name": "pincode",
          "type": "string",
          "description": "Correct PIN (digits only)",
          "required": true
        },
        {
          "name": "unlocked_content",
          "type": "markdown",
          "description": "Content shown after a correct PIN"
        }
      ]
    },
    {
      "type": "quiz",
      "name": "Quiz",
      "description": "Multiple choice question with configurable answer options.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "question",
          "type": "markdown",
          "description": "Question text",
          "required": true
        },
        {
          "name": "options",
          "type": "list",
          "description": "Answer choices",
          "required": true,
          "items": {
            "name": "",
            "type": "object",
            "fields": [
              {
                "name": "id",
                "type": "string",
                "description": "Unique option identifier"
              },
              {
                "name": "text",
                "type": "string",
                "description": "Option text",
                "required": true
              },
              {
                "name": "correct",
                "type": "bool",
                "description": "Whether this option is correct",
                "required": true
              }
            ]
          }
        },
        {
          "name": "multiple_choice",
          "type": "bool",
          "description": "Allow selecting multiple options"
        },
        {
          "name": "randomise_order",
          "type": "bool",
          "description": "Shuffle option order for each player"
        },
        {
          "name": "allow_retry",
          "type": "bool",
          "description": "Allow players to retry after an incorrect answer"
        },
        {
          "name": "unlocked_content",
          "type": "markdown",
          "description": "Content shown after a correct answer"
        }
      ]
    },
    {
      "type": "random_clue",
      "name": "Random Clue",
      "description": "Shows a randomly selected clue from a list, deterministically chosen per team.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "clues",
          "type": "list",
          "description": "List of clue strings to randomly select from",
          "required": true,
          "items": {
            "name": "",
            "type": "string"
          }
        }
      ]
    },
    {
      "type": "rating",
      "name": "Rating",
      "description": "Players rate something on a star scale.",
      "contexts": [
        "finish",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "What the player is rating"
        },
        {
          "name": "max_rating",
          "type": "int",
          "description": "Maximum rating value (default 5)"
        }
      ]
    },
    {
      "type": "scan",
      "name": "Scan",
      "description": "Players scan a QR code or barcode to proceed.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "Instruction shown to the player"
        },
        {
          "name": "codes",
          "type": "array",
          "description": "Values that satisfy the block; any one counts. A block with none can never be passed",
          "fields": [
            {
              "name": "value",
              "type": "string",
              "description": "The value the code carries",
              "required": true
            },
            {
              "name": "generate",
              "type": "bool",
              "description": "Render as a printable QR. Off for codes already in the world, like an ISBN",
              "default": "false"
            }
          ]
        },
        {
          "name": "match",
          "type": "string",
          "description": "How a scan is compared: exact is byte-for-byte, ci ignores case, contains accepts a value carrying the code and ignores case too, such as a QR encoding a URL",
          "default": "exact",
          "enum": [
            "exact",
            "ci",
            "contains"
          ]
        }
      ]
    },
    {
      "type": "sorting",
      "name": "Sorting",
      "description": "Players drag and drop items into the correct order.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when",
        "points",
        "sets"
      ],
      "fields": [
        {
          "name": "content",
          "type": "markdown",
          "description": "Instructions or question shown above the items"
        },
        {
          "name": "items",
          "type": "list",
          "description": "Items to sort",
          "required": true,
          "items": {
            "name": "",
            "type": "object",
            "fields": [
              {
                "name": "id",
                "type": "string",
                "description": "Unique item identifier"
              },
              {
                "name": "description",
                "type": "string",
                "description": "Item label",
                "required": true
              },
              {
                "name": "position",
                "type": "int",
                "description": "Correct 1-based position",
                "required": true
              }
            ]
          }
        },
        {
          "name": "scoring",
          "type": "enum",
          "description": "How points are awarded",
          "enum": [
            "all_or_nothing",
            "partial"
          ]
        }
      ]
    },
    {
      "type": "start_button",
      "name": "Start Game Button",
      "description": "The button players tap to join and start playing the game.",
      "contexts": [
        "start"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "scheduled_text",
          "type": "string",
          "description": "Button label shown before the game starts"
        },
        {
          "name": "active_text",
          "type": "string",
          "description": "Button label shown when the game is active"
        },
        {
          "name": "style",
          "type": "enum",
          "description": "Button visual style",
          "enum": [
            "primary",
            "secondary",
            "accent"
          ]
        }
      ]
    },
    {
      "type": "team_name",
      "name": "Team Name",
      "description": "Displays the team name and optionally allows players to change it.",
      "contexts": [
        "start"
      ],
      "shared_fields": [
        "when",
        "points"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "Label shown above the team name field"
        },
        {
          "name": "allow_changing",
          "type": "bool",
          "description": "If true, players can edit their team name"
        }
      ]
    },
    {
      "type": "text",
      "name": "Markdown",
      "description": "Renders Markdown content. Supports headings, lists, links, bold, italic, and images.",
      "contexts": [
        "finish",
        "start",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "content",
          "type": "markdown",
          "description": "Markdown text to display",
          "required": true
        }
      ]
    },
    {
      "type": "toggle_text",
      "name": "Toggle Text",
      "description": "Collapsible text section with a title and hidden content.",
      "contexts": [
        "start",
        "finish",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "title",
          "type": "string",
          "description": "Visible toggle button label",
          "required": true
        },
        {
          "name": "content",
          "type": "markdown",
          "description": "Hidden content revealed when toggled",
          "required": true
        },
        {
          "name": "small",
          "type": "bool",
          "description": "Use a smaller toggle style"
        }
      ]
    },
    {
      "type": "youtube",
      "name": "YouTube",
      "description": "Embeds a YouTube video.",
      "contexts": [
        "finish",
        "start",
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "when"
      ],
      "fields": [
        {
          "name": "url",
          "type": "string",
          "description": "YouTube video URL or embed URL",
          "required": true
        }
      ]
    }
  ]
}
```
