package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nathanhollows/Rapua/v8/internal/specgen"
	"github.com/urfave/cli/v2"
)

const gameSpecHeader = "---\ntitle: \"Game Spec\"\nsidebar: true\norder: 21\n---\n\n# Game Spec\n\n> Generated from code. Regenerate with: `rapua genspec`\n>\n> Machine-readable: `GET /api/v8/spec`\n\n## Authoring constraints\n\nThese rules are enforced by the linter (`POST /api/v8/lint`). Errors block import; warnings should be fixed.\n\n**Structure**\n- Every objective must be inside a group. An objective placed directly under `structure.children` is never shown to players. Wrap it in a group. *(`ROOT_OBJECTIVE_HIDDEN`)*\n- Groups must have at least one child. An empty group produces a warning. *(`EMPTY_GROUP`)*\n- Objective slugs must be unique across the entire game, including across groups. *(`SLUG_DUPLICATE`)*\n\n**Import modes**\n- **Create-import** (`POST /admin/quests/import`): omit `id` on objectives and blocks: new UUIDs are generated.\n- **Update-import** (`POST /admin/quests/{id}/import`): include `id` to reconcile with existing records. Matched blocks preserve player state (`RunBlockState`). Objectives absent from the document are deleted.\n- Group `id` is preserved on update-import to avoid orphaning team progress records (`SkippedGroupIDs`).\n\n**Blocks**\n- Every block must have a `type` field matching a registered block type.\n- A block may only appear in contexts listed in its spec. *(`INVALID_CONTEXT`)*\n- Block `id` values must be unique across the document. *(`BLOCK_ID_DUPLICATE`)*\n- Block `points` are ignored unless `settings.enable_points` is true. *(`POINTS_DISABLED` warning)* An objective has no `points` field of its own: its total point value is the sum of its blocks' points.\n\n**Start page**\n- A start page with blocks but no `start_button` block will not let players join. *(`NO_START_BUTTON` warning)*\n\n**Completion**\n- `minimum_required` is only valid when `completion` is `\"minimum\"`; it must be a positive integer. *(`MINIMUM_REQUIRED_MISMATCH` / `MINIMUM_REQUIRED_MISSING`)*\n\n**Reachability (`depends` / `sets`)**\n- `depends` is a flat list of variable names on an objective, implicitly ANDed. Each name is a truthy check: there are no comparison operators. Prefix a name with `not ` to negate it.\n- A name is either `objective.<slug>` or a variable written by a block or context `sets`. Anything else warns. *(`UNDEFINED_VAR`, `UNDEFINED_OBJECTIVE_VAR`)*\n- A `depends` entry that names no variable is an error. *(`DEPENDS_EMPTY_NAME`)*\n- `sets` is a list of variable names, each written as `\"true\"` when the block or context completes. Any other shape is an error. *(`SETS_NOT_LIST`)*\n- `sets` must not write to the runtime-owned `objective.*` namespace. *(`SETS_RESERVED_NAMESPACE`)*\n- `sets` on a content block (text, alert, image, etc.) is ignored. *(`SETS_ON_CONTENT_BLOCK` warning)*\n- A `sets` variable that no `depends` list references produces a warning. *(`UNUSED_VAR` warning)*\n\n## Full spec\n\n```json\n"

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
	content := gameSpecHeader + string(data) + gameSpecFooter
	if err := os.WriteFile(outPath, []byte(content), 0o600); err != nil {
		return 0, fmt.Errorf("write %s: %w", outPath, err)
	}
	return len(content), nil
}
