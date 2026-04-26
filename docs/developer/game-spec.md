---
title: "Game Spec"
sidebar: true
order: 21
---

# Game Spec

> Generated from code. Regenerate with: `rapua genspec`
>
> Machine-readable: `GET /api/v7/spec`

## Authoring constraints

These rules are enforced by the linter (`POST /api/v7/lint`). Errors block import; warnings should be fixed.

**Structure**
- Every location must be inside a group. A location placed directly under `structure.children` is never shown to players. Wrap it in a group. *(`ROOT_LOCATION_HIDDEN`)*
- Groups must have at least one child. An empty group produces a warning. *(`EMPTY_GROUP`)*
- Location slugs must be unique across the entire game, including across groups. *(`SLUG_DUPLICATE`)*

**Import modes**
- **Create-import** (`POST /admin/instances/import`): omit `id` on locations and blocks — new UUIDs are generated.
- **Update-import** (`POST /admin/instances/{id}/import`): include `id` to reconcile with existing records. Matched blocks preserve player state (`TeamBlockState`). Locations absent from the document are deleted.
- Group `id` is preserved on update-import to avoid orphaning team progress records (`SkippedGroupIDs`).

**Blocks**
- Every block must have a `type` field matching a registered block type.
- A block may only appear in contexts listed in its spec. *(`INVALID_CONTEXT`)*
- Block `id` values must be unique across the document. *(`BLOCK_ID_DUPLICATE`)*
- Block and location `points` are ignored unless `settings.enable_points` is true. *(`POINTS_DISABLED` warning)*

**Start page**
- A start page with blocks but no `start_button` block will not let players join. *(`NO_START_BUTTON` warning)*

**Completion**
- `minimum_required` is only valid when `completion` is `"minimum"`; it must be a positive integer. *(`MINIMUM_REQUIRED_MISMATCH` / `MINIMUM_REQUIRED_MISSING`)*

## Full spec

```json
{
  "version": "v7",
  "document": {
    "description": "Top-level v7 game document.",
    "fields": [
      {
        "name": "rapua",
        "type": "string",
        "description": "Format version. Must be \"v7\".",
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
            "name": "must_check_out",
            "type": "bool",
            "description": "Players must explicitly check out of each location before moving on."
          },
          {
            "name": "show_team_count",
            "type": "bool",
            "description": "Show how many teams are at each location."
          },
          {
            "name": "enable_points",
            "type": "bool",
            "description": "Enable the points system."
          },
          {
            "name": "enable_bonus_points",
            "type": "bool",
            "description": "Enable bonus points on blocks."
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
        "description": "Root group defining routing, navigation, and the location tree.",
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
            "description": "How players are routed through locations. See enums.routing.",
            "required": true
          },
          {
            "name": "navigation",
            "type": "enum",
            "description": "How the navigation UI is presented. See enums.navigation.",
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
            "description": "Number of locations required when completion is \"minimum\"."
          },
          {
            "name": "children",
            "type": "list",
            "description": "Ordered list of location or group children.",
            "required": true,
            "items": {
              "name": "",
              "type": "object",
              "description": "Tagged union: set exactly one of \"location\" or \"group\".",
              "fields": [
                {
                  "name": "location",
                  "type": "object",
                  "description": "A single game location. See location schema."
                },
                {
                  "name": "group",
                  "type": "object",
                  "description": "A named sub-group with its own routing/navigation/completion settings."
                }
              ]
            }
          }
        ]
      },
      {
        "name": "location",
        "type": "object",
        "description": "Schema for location objects within structure.children.",
        "fields": [
          {
            "name": "id",
            "type": "string",
            "description": "Location UUID. Present on export; omit on create-import to generate a new UUID."
          },
          {
            "name": "slug",
            "type": "string",
            "description": "Short alphanumeric code used in QR/URLs. Must be unique within the game.",
            "required": true
          },
          {
            "name": "name",
            "type": "string",
            "description": "Display name shown to players.",
            "required": true
          },
          {
            "name": "points",
            "type": "int",
            "description": "Points awarded on check-in (requires enable_points in settings)."
          },
          {
            "name": "marker",
            "type": "object",
            "description": "Geographic map pin. Omit if the location has no map position.",
            "fields": [
              {
                "name": "lat",
                "type": "float",
                "description": "Latitude in decimal degrees.",
                "required": true
              },
              {
                "name": "lng",
                "type": "float",
                "description": "Longitude in decimal degrees.",
                "required": true
              }
            ]
          },
          {
            "name": "content",
            "type": "list",
            "description": "Blocks shown to players after check-in. Always present, even if empty.",
            "required": true
          },
          {
            "name": "navigation",
            "type": "list",
            "description": "Blocks shown on the /next page to help players find this location."
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
        "description": "The game randomly selects locations for each player/team. Good for large groups."
      },
      {
        "value": "free_roam",
        "label": "Open Exploration",
        "description": "Players visit locations in any order. All locations are visible."
      },
      {
        "value": "ordered",
        "label": "Guided Path",
        "description": "Players must visit locations in a fixed order."
      },
      {
        "value": "secret",
        "label": "Secret",
        "description": "Locations never explicitly shown; players access them out of sequence."
      }
    ],
    "navigation": [
      {
        "value": "map",
        "label": "Map Only",
        "description": "Players are shown a map with unlabelled pins."
      },
      {
        "value": "labelled_map",
        "label": "Labelled Map",
        "description": "Players are shown a map with location names."
      },
      {
        "value": "location_list",
        "label": "Location List",
        "description": "Players are shown a list of location names."
      },
      {
        "value": "custom",
        "label": "Custom Clues",
        "description": "Players see custom content (e.g. randomised clues) built with the block builder."
      },
      {
        "value": "tasks",
        "label": "Tasks",
        "description": "Players see a scavenger-hunt-style checklist with completion tracking."
      }
    ],
    "completion": [
      {
        "value": "all",
        "label": "All",
        "description": "All locations/groups in this group must be completed."
      },
      {
        "value": "minimum",
        "label": "Minimum",
        "description": "At least minimum_required locations/groups must be completed."
      }
    ]
  },
  "contexts": [
    {
      "value": "location_content",
      "description": "Blocks shown to players after checking in to a location."
    },
    {
      "value": "navigation",
      "description": "Blocks shown on the /next page to help players find a location."
    },
    {
      "value": "start",
      "description": "Blocks shown on the game start page (introductions, rules, team name, start button)."
    },
    {
      "value": "finish",
      "description": "Blocks shown on the game finish/end page."
    }
  ],
  "blocks": [
    {
      "type": "alert",
      "name": "Alert",
      "description": "Displays a styled alert box with a message.",
      "contexts": [
        "location_content",
        "finish",
        "start"
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
      "description": "Players spend points to reveal progressively detailed information tiers.",
      "contexts": [
        "location_content",
        "navigation"
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
        "location_content",
        "finish",
        "start"
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
        "location_content",
        "start"
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
      "type": "clue",
      "name": "Clue",
      "description": "A clue revealed behind a button — players tap to reveal the hint.",
      "contexts": [
        "location_content",
        "navigation"
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
        "location_content",
        "finish",
        "start"
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
      "type": "header",
      "name": "Header",
      "description": "A page header with an icon and title.",
      "contexts": [
        "location_content",
        "start",
        "finish"
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
        "location_content",
        "navigation",
        "finish",
        "start"
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
        "location_content",
        "navigation",
        "finish",
        "start"
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
        "location_content"
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
        "location_content",
        "finish"
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
        "location_content"
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
        "location_content"
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
      "description": "Shows a randomly selected clue from a list each time the player views the location.",
      "contexts": [
        "navigation"
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
        "location_content",
        "finish"
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
      "type": "sorting",
      "name": "Sorting",
      "description": "Players drag and drop items into the correct order.",
      "contexts": [
        "location_content"
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
      "type": "task",
      "name": "Task",
      "description": "A single task item in a scavenger hunt task list.",
      "contexts": [
        "navigation"
      ],
      "fields": [
        {
          "name": "task",
          "type": "string",
          "description": "Task description shown to players",
          "required": true
        },
        {
          "name": "icon",
          "type": "string",
          "description": "Icon name (Lucide icon)"
        },
        {
          "name": "link_through",
          "type": "bool",
          "description": "If true, tapping takes the player to the task location"
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
        "location_content",
        "navigation",
        "finish",
        "start"
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
        "location_content",
        "navigation",
        "start",
        "finish"
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
        "location_content",
        "finish",
        "start"
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
