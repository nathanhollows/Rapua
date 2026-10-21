package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/specgen"
	"github.com/urfave/cli/v2"
)

const gameSpecHeader = "---\ntitle: \"Game Spec\"\nsidebar: true\norder: 21\n---\n\n# Game Spec\n\n> Generated from code. Regenerate with: `rapua genspec`\n>\n> Machine-readable: `GET /api/v8/spec`\n\n## Authoring constraints\n\nThese rules are enforced by the linter (`POST /api/v8/lint`). Errors block import; warnings should be fixed.\n\n**Structure**\n- The document is one recursive type. `structure` is the root objective, and every node under `children` has the same schema. An objective with children is a section; one without is a leaf. Nothing else distinguishes them.\n- Objective slugs must be unique across the whole document, root and sections included. *(`SLUG_DUPLICATE`)*\n- `routing` is required on an objective with children, and must name one of the three strategies: an empty value is an unmade choice, not a default. It is inert on an objective without children, which is not worth reporting: every objective carries one. *(`INVALID_ROUTING`)*\n- The player's screen draws at most {{MAX_DRAWN_DEPTH}} sections deep. A section nested further, below {{MAX_DRAWN_DEPTH}} that are drawn, passes through: it still routes and bands, but its tasks appear in the section above. Warns so the author can mark it `pass_through` on purpose; sections marked `pass_through` do not count toward the {{MAX_DRAWN_DEPTH}}. *(`FORCED_PASS_THROUGH`)*\n- A `pass_through` section needs `children_min` equal to `children_max`, because players never see its finish button. *(`PASS_THROUGH_RANGED_BAND`)* Asking for fewer children than it shows at once warns. *(`PASS_THROUGH_HIDDEN_CHOICE`)*\n\n**Import modes**\n- **Create-import** (`POST /admin/quests/import`): omit `id` on objectives and blocks: new UUIDs are generated.\n- **Update-import** (`POST /admin/quests/{id}/import`): include `id` to reconcile with existing records. Matched blocks preserve player state (`RunBlockState`). Objectives absent from the document are deleted.\n\n**Blocks**\n- Every block must have a `type` field matching a registered block type.\n- A block may only appear in contexts listed in its spec. *(`INVALID_CONTEXT`)*\n- Block `id` values must be unique across the document. *(`BLOCK_ID_DUPLICATE`)*\n- Block `points` are ignored unless `settings.enable_points` is true, and are left alone rather than reported: the editor hides every points control while points are off, so nothing could be done about it from there. An objective has no `points` field of its own: its total point value is the sum of its blocks' points.\n\n**Start page**\n- A start page with blocks but no `start_button` block will not let players join. *(`NO_START_BUTTON` warning)*\n\n**Completion band**\n- `children_min` and `children_max` form a range over completed children. When they are equal the objective auto-completes at that count; when min is lower, reaching min reveals a finish button and the player's press completes the objective, which also auto-completes at max.\n- Omitting both requires every child. Naming either bound widens the other to its extreme (min to 0, max to the child count), so an explicit `children_min: 0` is not the same as omitting it.\n- `children_min` must not exceed `children_max`. *(`BAND_MIN_EXCEEDS_MAX`)*\n- Both bounds must lie between 0 and the child count. *(`BAND_OUT_OF_RANGE`)*\n- `children_max: 0` completes the objective before any child is reachable. *(`BAND_COMPLETES_AT_ZERO`)*\n- The band, `max_next` and `finish_label` are inert on an objective with no children. *(`BAND_ON_LEAF`, `MAX_NEXT_ON_LEAF`, `FINISH_LABEL_UNREACHABLE`)*\n- `finish_label` only shows on an objective in a range. *(`FINISH_LABEL_UNREACHABLE`)*\n\n**Variables (`sets` / `depends`)**\n- A variable is a name that is either set or not. Names are lowercase letters, digits and underscores, not starting with a digit. *(`VAR_NAME_INVALID`)*\n- Only an interactive block sets a variable, the moment the player completes it, through its `sets` field (one name). A choice instead sets the variable of each option the player picks. Objectives and content blocks never set variables. A block completed outside the usual answer, such as naming the team, cannot set one either. *(`SETS_ON_STATIC_BLOCK`, `SETS_ON_CHOICE`, `SETS_UNSUPPORTED`, `SETS_NOT_STRING`)*\n- `depends` is a list of names that must all be set. On an objective it holds the objective locked; on a content block it hides the block entirely. An interactive block cannot depend on anything, and neither can the root. *(`DEPENDS_ON_INTERACTIVE_BLOCK`, `ROOT_DEPENDS`, `DEPENDS_NOT_LIST`, `DEPENDS_EMPTY_NAME`)*\n- There is no negation and no comparison: a gate that has opened stays open.\n- Every name in a `depends` list must be set by some block. *(`UNDEFINED_VAR`)* A name only set inside a draft warns. *(`VAR_ONLY_SET_IN_DRAFT` warning)* A `sets` nothing depends on warns. *(`UNUSED_VAR` warning)*\n- A gate whose variables are only set behind that gate, or behind another gate waiting on it, can never open. Ordered routing counts: a child cannot wait on a sibling that comes after it. *(`DEPENDS_UNSATISFIABLE`)*\n- A single-select choice sets one option, so one gate cannot need two of its options *(`DEPENDS_EXCLUSIVE_CHOICES`)*, and a section that needs children gated on different options cannot be completed *(`BAND_NEEDS_EXCLUSIVE_CHOICES` warning)*.\n- A proof block waiting on something set only after its proof clears is never seen. *(`BLOCK_DEPENDS_TOO_LATE` warning)* A gated child in a randomised section with `max_next` can take a place in the window while shut. *(`DEPENDS_IN_RANDOMISED_WINDOW` warning)* A gated child whose variables are set only by its siblings, in a section that closes before that many are done, will likely stay locked. *(`BAND_CLOSES_FIRST` warning)*\n\n\n## Full spec\n\n```json\n"

const gameSpecFooter = "\n```\n"

func newGenSpecCommand() *cli.Command {
	return &cli.Command{
		Name:  "genspec",
		Usage: "regenerate docs/developer/game-spec.md from code",
		Action: func(_ *cli.Context) error {
			out := filepath.Join("docs", "developer", "game-spec.md")
			n, err := writeGameSpec(out)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "wrote %s (%d bytes)\n", out, n)
			return nil
		},
	}
}

// writeGameSpec generates the game spec and writes it to outPath.
// Returns the number of bytes written.
func writeGameSpec(outPath string) (int, error) {
	data, err := specgen.GenerateJSON()
	if err != nil {
		return 0, fmt.Errorf("generate spec: %w", err)
	}
	header := strings.ReplaceAll(gameSpecHeader, "{{MAX_DRAWN_DEPTH}}", strconv.Itoa(game.MaxDrawnDepth))
	content := header + string(data) + gameSpecFooter
	if err := os.WriteFile(outPath, []byte(content), 0o600); err != nil {
		return 0, fmt.Errorf("write %s: %w", outPath, err)
	}
	return len(content), nil
}
