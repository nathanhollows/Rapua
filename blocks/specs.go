package blocks

import "github.com/nathanhollows/Rapua/v8/game"

// GetSpec implementations for all block types.
// These satisfy game.SpecProvider and are used by internal/specgen.

func (b *MarkdownBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "text",
		Name:        "Markdown",
		Description: "Renders Markdown content. Supports headings, lists, links, bold, italic, and images.",
		Fields: []game.FieldSpec{
			{Name: "content", Type: "markdown", Required: true, Description: "Markdown text to display"},
		},
	}
}

func (b *AlertBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "alert",
		Name:        "Alert",
		Description: "Displays a styled alert box with a message.",
		Fields: []game.FieldSpec{
			{Name: "content", Type: "string", Required: true, Description: "Alert message text"},
			{Name: "style", Type: "enum", Description: "Visual style of the alert",
				Enum: []string{"info", "success", "warning", "error"}},
		},
	}
}

func (b *ButtonBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "button",
		Name:        "Button",
		Description: "A clickable button that opens a URL.",
		Fields: []game.FieldSpec{
			{Name: "link", Type: "string", Required: true, Description: "URL the button opens"},
			{Name: "text", Type: "string", Required: true, Description: "Button label text"},
			{Name: "style", Type: "enum", Description: "Button visual style",
				Enum: []string{"primary", "secondary", "accent", "ghost"}},
		},
	}
}

func (b *BrokerBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "broker",
		Name:        "Information Broker",
		Description: "Players spend points to reveal progressively detailed information tiers.",
		Fields: []game.FieldSpec{
			{Name: "prompt", Type: "string", Description: "Instructions shown to the player"},
			{Name: "default_info", Type: "markdown", Description: "Information shown for free (0 points)"},
			{Name: "tiers", Type: "list", Description: "Paid information tiers in ascending cost order",
				Items: &game.FieldSpec{
					Type: "object",
					Fields: []game.FieldSpec{
						{
							Name:        "points_required",
							Type:        "int",
							Required:    true,
							Description: "Points cost for this tier",
						},
						{
							Name:        "content",
							Type:        "markdown",
							Required:    true,
							Description: "Information revealed at this tier",
						},
					},
				}},
		},
	}
}

func (b *ChecklistBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "checklist",
		Name:        "Checklist",
		Description: "An interactive checklist players can tick off.",
		Fields: []game.FieldSpec{
			{Name: "content", Type: "markdown", Description: "Optional introductory text above the checklist"},
			{Name: "items", Type: "list", Required: true, Description: "Checklist items",
				Items: &game.FieldSpec{
					Type: "object",
					Fields: []game.FieldSpec{
						{Name: "id", Type: "string", Description: "Unique item identifier"},
						{Name: "description", Type: "string", Required: true, Description: "Item text"},
						{Name: "checked", Type: "bool", Description: "Default checked state"},
					},
				}},
		},
	}
}

func (b *ClueBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "clue",
		Name:        "Clue",
		Description: "A clue revealed behind a button — players tap to reveal the hint.",
		Fields: []game.FieldSpec{
			{
				Name:        "clue",
				Type:        "markdown",
				Required:    true,
				Description: "Clue content revealed when the button is tapped",
			},
			{Name: "description", Type: "string", Description: "Description or label shown on the reveal button"},
			{Name: "button_label", Type: "string", Description: "Custom button label (defaults to 'Reveal clue')"},
		},
	}
}

func (b *DividerBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "divider",
		Name:        "Divider",
		Description: "A horizontal rule with an optional title label.",
		Fields: []game.FieldSpec{
			{Name: "title", Type: "string", Description: "Optional label shown on the divider"},
		},
	}
}

func (b *GameStatusAlertBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "game_status",
		Name:        "Game Status Alert",
		Description: "Shows the current game status (scheduled, active, or closed) with optional messages.",
		Fields: []game.FieldSpec{
			{Name: "closed_message", Type: "string", Description: "Message shown when the game is closed"},
			{Name: "scheduled_message", Type: "string", Description: "Message shown when the game is scheduled"},
			{Name: "show_countdown", Type: "bool", Description: "Show a countdown timer when game is scheduled"},
		},
	}
}

func (b *HeaderBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "header",
		Name:        "Header",
		Description: "A page header with an icon and title.",
		Fields: []game.FieldSpec{
			{Name: "icon", Type: "string", Description: "Icon name (Lucide icon)"},
			{Name: "title", Type: "string", Required: true, Description: "Header title text"},
			{Name: "size", Type: "enum", Description: "Title font size",
				Enum: []string{"sm", "md", "lg", "xl", "2xl", "3xl"}},
		},
	}
}

func (b *ImageBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "image",
		Name:        "Image",
		Description: "Displays an image with an optional caption and link.",
		Fields: []game.FieldSpec{
			{Name: "url", Type: "string", Required: true, Description: "Image URL"},
			{Name: "caption", Type: "string", Description: "Caption displayed below the image"},
			{Name: "link", Type: "string", Description: "Optional URL to open when the image is clicked"},
			{Name: "full_width", Type: "bool", Description: "Render image edge-to-edge on mobile screens"},
		},
	}
}

func (b *PasswordBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "password",
		Name:        "Password",
		Description: "Players must enter the correct text answer to proceed.",
		Fields: []game.FieldSpec{
			{Name: "prompt", Type: "string", Description: "Question or instruction shown to the player"},
			{Name: "answer", Type: "string", Required: true, Description: "Correct answer (case-insensitive)"},
			{Name: "unlocked_content", Type: "markdown", Description: "Content shown after a correct answer"},
		},
	}
}

func (b *ScanBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "scan",
		Name:        "Scan",
		Description: "Players scan a QR code or barcode to proceed.",
		Fields: []game.FieldSpec{
			{Name: "prompt", Type: "string", Description: "Instruction shown to the player"},
			{
				Name: "codes", Type: "array",
				Description: "Values that satisfy the block; any one counts. " +
					"A block with none can never be passed",
				Fields: []game.FieldSpec{
					{Name: "value", Type: "string", Required: true, Description: "The value the code carries"},
					{
						Name: "generate", Type: "bool", Default: "false",
						Description: "Render as a printable QR. Off for codes already in the world, like an ISBN",
					},
				},
			},
			{
				Name: "match", Type: "string", Default: "exact",
				Enum: []string{"exact", "ci", "contains"},
				Description: "How a scan is compared: exact is byte-for-byte, ci ignores case, " +
					"contains accepts a value carrying the code and ignores case too, " +
					"such as a QR encoding a URL",
			},
		},
	}
}

func (b *PhotoBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "photo",
		Name:        "Photo Upload",
		Description: "Players upload one or more photos as their response.",
		Fields: []game.FieldSpec{
			{Name: "prompt", Type: "string", Description: "Instructions shown to the player"},
			{Name: "max_images", Type: "int", Description: "Maximum number of photos the player can upload"},
		},
	}
}

func (b *PincodeBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "pincode",
		Name:        "Pin Code",
		Description: "Players must enter a numeric PIN to proceed.",
		Fields: []game.FieldSpec{
			{Name: "prompt", Type: "string", Description: "Question or instruction shown to the player"},
			{Name: "pincode", Type: "string", Required: true, Description: "Correct PIN (digits only)"},
			{Name: "unlocked_content", Type: "markdown", Description: "Content shown after a correct PIN"},
		},
	}
}

func (b *QuizBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "quiz",
		Name:        "Quiz",
		Description: "Multiple choice question with configurable answer options.",
		Fields: []game.FieldSpec{
			{Name: "question", Type: "markdown", Required: true, Description: "Question text"},
			{Name: "options", Type: "list", Required: true, Description: "Answer choices",
				Items: &game.FieldSpec{
					Type: "object",
					Fields: []game.FieldSpec{
						{Name: "id", Type: "string", Description: "Unique option identifier"},
						{Name: "text", Type: "string", Required: true, Description: "Option text"},
						{Name: "correct", Type: "bool", Required: true, Description: "Whether this option is correct"},
					},
				}},
			{Name: "multiple_choice", Type: "bool", Description: "Allow selecting multiple options"},
			{Name: "randomise_order", Type: "bool", Description: "Shuffle option order for each player"},
			{Name: "allow_retry", Type: "bool", Description: "Allow players to retry after an incorrect answer"},
			{Name: "unlocked_content", Type: "markdown", Description: "Content shown after a correct answer"},
		},
	}
}

func (b *RandomClueBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "random_clue",
		Name:        "Random Clue",
		Description: "Shows a randomly selected clue from a list, deterministically chosen per team.",
		Fields: []game.FieldSpec{
			{Name: "clues", Type: "list", Required: true, Description: "List of clue strings to randomly select from",
				Items: &game.FieldSpec{Type: "string"}},
		},
	}
}

func (b *RatingBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "rating",
		Name:        "Rating",
		Description: "Players rate something on a star scale.",
		Fields: []game.FieldSpec{
			{Name: "prompt", Type: "string", Description: "What the player is rating"},
			{Name: "max_rating", Type: "int", Description: "Maximum rating value (default 5)"},
		},
	}
}

func (b *SortingBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "sorting",
		Name:        "Sorting",
		Description: "Players drag and drop items into the correct order.",
		Fields: []game.FieldSpec{
			{Name: "content", Type: "markdown", Description: "Instructions or question shown above the items"},
			{Name: "items", Type: "list", Required: true, Description: "Items to sort",
				Items: &game.FieldSpec{
					Type: "object",
					Fields: []game.FieldSpec{
						{Name: "id", Type: "string", Description: "Unique item identifier"},
						{Name: "description", Type: "string", Required: true, Description: "Item label"},
						{Name: "position", Type: "int", Required: true, Description: "Correct 1-based position"},
					},
				}},
			{Name: "scoring", Type: "enum", Description: "How points are awarded",
				Enum: []string{"all_or_nothing", "partial"}},
		},
	}
}

func (b *StartGameButtonBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "start_button",
		Name:        "Start Game Button",
		Description: "The button players tap to join and start playing the game.",
		Fields: []game.FieldSpec{
			{Name: "scheduled_text", Type: "string", Description: "Button label shown before the game starts"},
			{Name: "active_text", Type: "string", Description: "Button label shown when the game is active"},
			{Name: "style", Type: "enum", Description: "Button visual style",
				Enum: []string{"primary", "secondary", "accent"}},
		},
	}
}

func (b *TeamNameChangerBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "team_name",
		Name:        "Team Name",
		Description: "Displays the team name and optionally allows players to change it.",
		Fields: []game.FieldSpec{
			{Name: "prompt", Type: "string", Description: "Label shown above the team name field"},
			{Name: "allow_changing", Type: "bool", Description: "If true, players can edit their team name"},
		},
	}
}

func (b *ToggleTextBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "toggle_text",
		Name:        "Toggle Text",
		Description: "Collapsible text section with a title and hidden content.",
		Fields: []game.FieldSpec{
			{Name: "title", Type: "string", Required: true, Description: "Visible toggle button label"},
			{Name: "content", Type: "markdown", Required: true, Description: "Hidden content revealed when toggled"},
			{Name: "small", Type: "bool", Description: "Use a smaller toggle style"},
		},
	}
}

func (b *MapBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "map",
		Name:        "Map",
		Description: "Displays a Mapbox map centred on a specific location with a marker.",
		Fields: []game.FieldSpec{
			{Name: "latitude", Type: "float", Required: true, Description: "Map centre latitude"},
			{Name: "longitude", Type: "float", Required: true, Description: "Map centre longitude"},
			{Name: "zoom", Type: "float", Description: "Map zoom level (1–20, default 14)"},
			{Name: "caption", Type: "string", Description: "Optional caption displayed below the map"},
			{Name: "hide_marker", Type: "bool", Description: "When true, the map pin is hidden on the player view"},
		},
	}
}

func (b *FreeTextBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "free_text",
		Name:        "Free Text",
		Description: "An ungraded text input for player reflections and free-form responses.",
		Fields: []game.FieldSpec{
			{Name: "prompt", Type: "string", Description: "Question or instruction shown to the player"},
			{Name: "placeholder", Type: "string", Description: "Placeholder text for the input field"},
		},
	}
}

func (b *ChoiceBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "choice",
		Name:        "Choice",
		Description: "Presents labelled options; selecting one sets a boolean variable.",
		Fields: []game.FieldSpec{
			{Name: "prompt", Type: "string", Required: true,
				Description: "Question or instruction shown above the choices"},
			{Name: "button_text", Type: "string", Required: false,
				Description: "Label for the submit button (default: \"Confirm choice\")"},
			{Name: "multi_select", Type: "bool", Required: false,
				Description: "Allow selecting multiple options (default: false — single choice)"},
			{Name: "options", Type: "list", Required: true,
				Description: "Choices presented to the player",
				Items: &game.FieldSpec{
					Type: "object",
					Fields: []game.FieldSpec{
						{Name: "label", Type: "string", Required: true,
							Description: "Display text for this choice"},
						{Name: "sets", Type: "string", Required: true,
							Description: "Variable name set to \"true\" when this choice is selected"},
					},
				}},
		},
	}
}

func (b *YoutubeBlock) GetSpec() game.BlockSpec {
	return game.BlockSpec{
		Type:        "youtube",
		Name:        "YouTube",
		Description: "Embeds a YouTube video.",
		Fields: []game.FieldSpec{
			{Name: "url", Type: "string", Required: true, Description: "YouTube video URL or embed URL"},
		},
	}
}
