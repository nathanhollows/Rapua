package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nathanhollows/Rapua/v7/internal/specgen"
	"github.com/urfave/cli/v2"
)

const gameSpecHeader = "---\ntitle: \"Game Spec\"\nsidebar: true\norder: 21\n---\n\n# Game Spec\n\n> Generated from code. Regenerate with: `rapua genspec`\n>\n> Machine-readable: `GET /api/v7/spec`\n\n## Authoring constraints\n\nThese rules are enforced by the linter (`POST /api/v7/lint`). Errors block import; warnings should be fixed.\n\n**Structure**\n- Every location must be inside a group. A location placed directly under `structure.children` is never shown to players. Wrap it in a group. *(`ROOT_LOCATION_HIDDEN`)*\n- Groups must have at least one child. An empty group produces a warning. *(`EMPTY_GROUP`)*\n- Location slugs must be unique across the entire game, including across groups. *(`SLUG_DUPLICATE`)*\n\n**Import modes**\n- **Create-import** (`POST /admin/instances/import`): omit `id` on locations and blocks — new UUIDs are generated.\n- **Update-import** (`POST /admin/instances/{id}/import`): include `id` to reconcile with existing records. Matched blocks preserve player state (`TeamBlockState`). Locations absent from the document are deleted.\n- Group `id` is preserved on update-import to avoid orphaning team progress records (`SkippedGroupIDs`).\n\n**Blocks**\n- Every block must have a `type` field matching a registered block type.\n- A block may only appear in contexts listed in its spec. *(`INVALID_CONTEXT`)*\n- Block `id` values must be unique across the document. *(`BLOCK_ID_DUPLICATE`)*\n- Block and location `points` are ignored unless `settings.enable_points` is true. *(`POINTS_DISABLED` warning)*\n\n**Start page**\n- A start page with blocks but no `start_button` block will not let players join. *(`NO_START_BUTTON` warning)*\n\n**Completion**\n- `minimum_required` is only valid when `completion` is `\"minimum\"`; it must be a positive integer. *(`MINIMUM_REQUIRED_MISMATCH` / `MINIMUM_REQUIRED_MISSING`)*\n\n**Conditional visibility (`when` / `sets`)**\n- Every variable referenced in a `when` condition must be defined in a block `sets` or the built-in variable list. *(`UNDEFINED_VAR`)*\n- No two `sets` declarations across the whole game may write the same variable name. *(`DUPLICATE_SETS_VAR`)*\n- `sets` variable names must not shadow built-in variable names. *(`SHADOWED_VAR`)*\n- `sets` is a list of variable names set to `\"true\"` when the block completes.\n- `op` in a condition must be a valid operator from `enums.condition_ops`. *(`INVALID_CONDITION_OP`)*\n- Every condition must have a `var` field; `value` is required when `op` is present. *(`INVALID_CONDITION`)*\n- `sets` on a content block (text, alert, image, etc.) is ignored. *(`SETS_ON_CONTENT_BLOCK` warning)*\n- A `sets` variable that is never referenced in any `when` clause produces a warning. *(`UNUSED_SETS_VAR` warning)*\n\n## Full spec\n\n```json\n"

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
