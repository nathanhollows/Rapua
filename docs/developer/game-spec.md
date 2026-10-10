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
- The document is one recursive type. `structure` is the root objective, and every node under `children` has the same schema. An objective with children is a section; one without is a leaf. Nothing else distinguishes them.
- Objective slugs must be unique across the whole document, root and sections included. *(`SLUG_DUPLICATE`)*
- `routing` is required on an objective with children, and must name one of the three strategies: an empty value is an unmade choice, not a default. It is inert on an objective without children, which is not worth reporting: every objective carries one. *(`INVALID_ROUTING`)*
- Nesting deeper than 4 levels warns: it is hard to navigate on a phone. *(`NESTING_TOO_DEEP`)*

**Import modes**
- **Create-import** (`POST /admin/quests/import`): omit `id` on objectives and blocks: new UUIDs are generated.
- **Update-import** (`POST /admin/quests/{id}/import`): include `id` to reconcile with existing records. Matched blocks preserve player state (`RunBlockState`). Objectives absent from the document are deleted.

**Blocks**
- Every block must have a `type` field matching a registered block type.
- A block may only appear in contexts listed in its spec. *(`INVALID_CONTEXT`)*
- Block `id` values must be unique across the document. *(`BLOCK_ID_DUPLICATE`)*
- Block `points` are ignored unless `settings.enable_points` is true, and are left alone rather than reported: the editor hides every points control while points are off, so nothing could be done about it from there. An objective has no `points` field of its own: its total point value is the sum of its blocks' points.

**Start page**
- A start page with blocks but no `start_button` block will not let players join. *(`NO_START_BUTTON` warning)*

**Completion band**
- `children_min` and `children_max` form a range over completed children. When they are equal the objective auto-completes at that count; when min is lower, reaching min reveals a finish button and the player's press completes the objective, which also auto-completes at max.
- Omitting both requires every child. Naming either bound widens the other to its extreme (min to 0, max to the child count), so an explicit `children_min: 0` is not the same as omitting it.
- `children_min` must not exceed `children_max`. *(`BAND_MIN_EXCEEDS_MAX`)*
- Both bounds must lie between 0 and the child count. *(`BAND_OUT_OF_RANGE`)*
- `children_max: 0` completes the objective before any child is reachable. *(`BAND_COMPLETES_AT_ZERO`)*
- The band, `max_next` and `finish_label` are inert on an objective with no children. *(`BAND_ON_LEAF`, `MAX_NEXT_ON_LEAF`, `FINISH_LABEL_UNREACHABLE`)*
- `finish_label` only shows on an objective in a range. *(`FINISH_LABEL_UNREACHABLE`)*


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
        "description": "Quest UUID. Present on export; omit on create-import to generate a new UUID."
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
        "description": "The root objective. An ordinary objective with no parent, using the schema below: the tree is one recursive type, not a container wrapping a different one.",
        "required": true,
        "fields": [
          {
            "name": "id",
            "type": "string",
            "description": "Objective UUID. Present on export; omit on create-import to generate a new UUID."
          },
          {
            "name": "slug",
            "type": "string",
            "description": "Short alphanumeric code identifying this objective. Must be unique within the game.",
            "required": true
          },
          {
            "name": "title",
            "type": "string",
            "description": "What the player is asked to do, shown on the quest list and at the top of the page they open. The task, not the place: \"Find the plan chest\" rather than \"Architecture Library\".",
            "required": true
          },
          {
            "name": "description",
            "type": "string",
            "description": "The detail a player needs once they are there, shown under the title. What will not fit in a name short enough to read in a list."
          },
          {
            "name": "draft",
            "type": "bool",
            "description": "Holds this objective out of play along with everything beneath it, without moving any of them: it keeps its place and its children, so publishing restores exactly what was there. The flag is never written downward, so a child of a draft carries none of its own. Omitting the key leaves an existing objective's state alone; only an explicit false publishes one. The root may not be a draft (ROOT_DRAFT), and a section whose children are all drafts behaves as a leaf (ALL_CHILDREN_DRAFT)."
          },
          {
            "name": "proof",
            "type": "object",
            "description": "Blocks shown while the objective is unproven. A non-empty proof must contain at least one interactive block, or it gates nothing. Proof gates children too: nothing below this objective is reachable until its proof clears. Clearing the proof is what completes an objective, which is what the frontier, the journal and the leaderboard all count. The root carries no proof or reveal content of its own (ROOT_HAS_CONTENT).",
            "required": true,
            "fields": [
              {
                "name": "blocks",
                "type": "list",
                "description": "Blocks shown to players while this context is active."
              }
            ]
          },
          {
            "name": "reveal",
            "type": "object",
            "description": "Blocks shown once proof completes. On a section this is its own content, and the section is offered to a player until they have seen it, then steps back and its children are offered in its place.",
            "required": true,
            "fields": [
              {
                "name": "blocks",
                "type": "list",
                "description": "Blocks shown to players while this context is active."
              }
            ]
          },
          {
            "name": "routing",
            "type": "enum",
            "description": "How players are routed through this objective's children. Required when there are children, meaningless without them. See enums.routing."
          },
          {
            "name": "children_min",
            "type": "int",
            "description": "How many children must complete before the player may finish this objective. The pair forms a completion band. When min equals max the objective auto-completes at that count with no player action. When min is lower, reaching min only reveals a finish button and the player's press is what completes the objective, which also auto-completes at max. Omitting both means every child is required ([n, n]). Naming either bound widens the other to its extreme: an omitted min is 0, an omitted max is the child count. So an explicit children_min of 0 is not the same as omitting it. min greater than max is an error (BAND_MIN_EXCEEDS_MAX), as is either bound outside 0..child count (BAND_OUT_OF_RANGE). Both are meaningless on an objective with no children (BAND_ON_LEAF)."
          },
          {
            "name": "children_max",
            "type": "int",
            "description": "How many completed children finish this objective on their own. The pair forms a completion band. When min equals max the objective auto-completes at that count with no player action. When min is lower, reaching min only reveals a finish button and the player's press is what completes the objective, which also auto-completes at max. Omitting both means every child is required ([n, n]). Naming either bound widens the other to its extreme: an omitted min is 0, an omitted max is the child count. So an explicit children_min of 0 is not the same as omitting it. min greater than max is an error (BAND_MIN_EXCEEDS_MAX), as is either bound outside 0..child count (BAND_OUT_OF_RANGE). Both are meaningless on an objective with no children (BAND_ON_LEAF)."
          },
          {
            "name": "max_next",
            "type": "int",
            "description": "How many children a randomised objective offers at once. 0 means all of them."
          },
          {
            "name": "finish_label",
            "type": "string",
            "description": "Label for the finish button. Only an objective in a range ever shows one, so setting this where children_min equals children_max warns (FINISH_LABEL_UNREACHABLE)."
          },
          {
            "name": "children",
            "type": "list",
            "description": "Ordered list of child objectives, each with this same schema. An objective with children is a section; one without is a leaf. Nothing else distinguishes them."
          }
        ]
      },
      {
        "name": "objective",
        "type": "object",
        "description": "Schema for every objective, root and children alike. Has no points field of its own; its total point value is the sum of its blocks' points.",
        "fields": [
          {
            "name": "id",
            "type": "string",
            "description": "Objective UUID. Present on export; omit on create-import to generate a new UUID."
          },
          {
            "name": "slug",
            "type": "string",
            "description": "Short alphanumeric code identifying this objective. Must be unique within the game.",
            "required": true
          },
          {
            "name": "title",
            "type": "string",
            "description": "What the player is asked to do, shown on the quest list and at the top of the page they open. The task, not the place: \"Find the plan chest\" rather than \"Architecture Library\".",
            "required": true
          },
          {
            "name": "description",
            "type": "string",
            "description": "The detail a player needs once they are there, shown under the title. What will not fit in a name short enough to read in a list."
          },
          {
            "name": "draft",
            "type": "bool",
            "description": "Holds this objective out of play along with everything beneath it, without moving any of them: it keeps its place and its children, so publishing restores exactly what was there. The flag is never written downward, so a child of a draft carries none of its own. Omitting the key leaves an existing objective's state alone; only an explicit false publishes one. The root may not be a draft (ROOT_DRAFT), and a section whose children are all drafts behaves as a leaf (ALL_CHILDREN_DRAFT)."
          },
          {
            "name": "proof",
            "type": "object",
            "description": "Blocks shown while the objective is unproven. A non-empty proof must contain at least one interactive block, or it gates nothing. Proof gates children too: nothing below this objective is reachable until its proof clears. Clearing the proof is what completes an objective, which is what the frontier, the journal and the leaderboard all count. The root carries no proof or reveal content of its own (ROOT_HAS_CONTENT).",
            "required": true,
            "fields": [
              {
                "name": "blocks",
                "type": "list",
                "description": "Blocks shown to players while this context is active."
              }
            ]
          },
          {
            "name": "reveal",
            "type": "object",
            "description": "Blocks shown once proof completes. On a section this is its own content, and the section is offered to a player until they have seen it, then steps back and its children are offered in its place.",
            "required": true,
            "fields": [
              {
                "name": "blocks",
                "type": "list",
                "description": "Blocks shown to players while this context is active."
              }
            ]
          },
          {
            "name": "routing",
            "type": "enum",
            "description": "How players are routed through this objective's children. Required when there are children, meaningless without them. See enums.routing."
          },
          {
            "name": "children_min",
            "type": "int",
            "description": "How many children must complete before the player may finish this objective. The pair forms a completion band. When min equals max the objective auto-completes at that count with no player action. When min is lower, reaching min only reveals a finish button and the player's press is what completes the objective, which also auto-completes at max. Omitting both means every child is required ([n, n]). Naming either bound widens the other to its extreme: an omitted min is 0, an omitted max is the child count. So an explicit children_min of 0 is not the same as omitting it. min greater than max is an error (BAND_MIN_EXCEEDS_MAX), as is either bound outside 0..child count (BAND_OUT_OF_RANGE). Both are meaningless on an objective with no children (BAND_ON_LEAF)."
          },
          {
            "name": "children_max",
            "type": "int",
            "description": "How many completed children finish this objective on their own. The pair forms a completion band. When min equals max the objective auto-completes at that count with no player action. When min is lower, reaching min only reveals a finish button and the player's press is what completes the objective, which also auto-completes at max. Omitting both means every child is required ([n, n]). Naming either bound widens the other to its extreme: an omitted min is 0, an omitted max is the child count. So an explicit children_min of 0 is not the same as omitting it. min greater than max is an error (BAND_MIN_EXCEEDS_MAX), as is either bound outside 0..child count (BAND_OUT_OF_RANGE). Both are meaningless on an objective with no children (BAND_ON_LEAF)."
          },
          {
            "name": "max_next",
            "type": "int",
            "description": "How many children a randomised objective offers at once. 0 means all of them."
          },
          {
            "name": "finish_label",
            "type": "string",
            "description": "Label for the finish button. Only an objective in a range ever shows one, so setting this where children_min equals children_max warns (FINISH_LABEL_UNREACHABLE)."
          },
          {
            "name": "children",
            "type": "list",
            "description": "Ordered list of child objectives, each with this same schema. An objective with children is a section; one without is a leaf. Nothing else distinguishes them."
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
      }
    ]
  },
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
      "type": "button",
      "name": "Button",
      "description": "A clickable button that opens a URL.",
      "contexts": [
        "finish",
        "start",
        "objective_proof",
        "objective_reveal"
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
        "points"
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
      "description": "Presents labelled options; the chosen one is recorded in player state.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "points"
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
          "description": "Allow selecting multiple options (default: false, a single choice)"
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
                "description": "Identifies this option. Recorded in player state when the option is chosen, and used to find its label again",
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
      "description": "A clue revealed behind a button: players tap to reveal the hint.",
      "contexts": [
        "objective_proof",
        "objective_reveal"
      ],
      "shared_fields": [
        "points"
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
        "points"
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
      "type": "geofence",
      "name": "Location check",
      "description": "Players must be standing in a place for this to pass.",
      "contexts": [
        "objective_proof"
      ],
      "shared_fields": [
        "points"
      ],
      "fields": [
        {
          "name": "prompt",
          "type": "string",
          "description": "Instruction shown to the player"
        },
        {
          "name": "area",
          "type": "object",
          "description": "Where the player must be: a circle with a centre and a radius in metres, or a GeoJSON Polygon"
        },
        {
          "name": "max_accuracy",
          "type": "float",
          "description": "Widest margin in metres still worth testing. A vaguer reading is refused rather than guessed at",
          "default": "50"
        },
        {
          "name": "map_mode",
          "type": "enum",
          "description": "What the player sees while checking: \"button\" hides the area (finding it is the challenge), \"map\" shows their live position, \"map_area\" also draws the target area",
          "default": "button",
          "enum": [
            "button",
            "map",
            "map_area"
          ]
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
        "points"
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
        "points"
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
        "points"
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
        "points"
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
        "points"
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
        "points"
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
        "points"
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
