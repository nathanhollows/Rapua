package game

import (
	"encoding/json"
	"fmt"
)

// LintJSON validates a raw JSON game document.
// It checks for unknown fields at the structural level (GameDoc, SettingsDoc,
// ObjectiveDoc) that are silently dropped by the JSON parser, then runs the
// full Lint pass on the parsed document.
func LintJSON(data []byte, registry BlockRegistry) LintResult {
	rawWarnings := checkUnknownFieldsRaw(data)

	var doc GameDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return LintResult{
			Errors: []LintDiag{{Path: "", Code: "INVALID_JSON", Message: err.Error()}},
		}
	}

	result := Lint(&doc, registry)
	result.Warnings = append(rawWarnings, result.Warnings...)
	return result
}

// Known JSON field sets for each structural type.
// These maps are effectively constant; they are never mutated after init.
//
//nolint:gochecknoglobals // lookup tables initialised once, never written after init
var (
	knownGameDocFields = map[string]bool{
		"rapua": true, "id": true, "name": true,
		"settings": true, "start": true, "finish": true, "structure": true,
	}
	knownSettingsDocFields = map[string]bool{
		"show_team_count": true, "enable_points": true,
		"show_leaderboard": true,
	}
	knownObjectiveDocFields = map[string]bool{
		"id": true, "slug": true, "title": true, "color": true,
		"depends": true, "proof": true, "reveal": true,
		"routing": true, "children_min": true, "children_max": true,
		"max_next": true, "finish_label": true, "children": true,
	}
	knownObjectiveContextDocFields = map[string]bool{"blocks": true, "sets": true}
)

func checkUnknownFieldsRaw(data []byte) []LintDiag {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var diags []LintDiag
	warnUnknown("", raw, knownGameDocFields, &diags)
	if s, ok := raw["settings"].(map[string]any); ok {
		warnUnknown("settings", s, knownSettingsDocFields, &diags)
	}
	if st, ok := raw["structure"].(map[string]any); ok {
		checkUnknownObjective("structure", st, &diags)
	}
	return diags
}

// checkUnknownObjective walks the node tree, which is the same shape at every
// level including the root.
func checkUnknownObjective(path string, obj map[string]any, diags *[]LintDiag) {
	warnUnknown(path, obj, knownObjectiveDocFields, diags)

	for _, ctx := range []string{"proof", "reveal"} {
		if fields, ok := obj[ctx].(map[string]any); ok {
			warnUnknown(path+"."+ctx, fields, knownObjectiveContextDocFields, diags)
		}
	}

	children, ok := obj["children"].([]any)
	if !ok {
		return
	}
	for i, child := range children {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		checkUnknownObjective(fmt.Sprintf("%s.children[%d]", path, i), childMap, diags)
	}
}

func warnUnknown(path string, obj map[string]any, known map[string]bool, diags *[]LintDiag) {
	for k := range obj {
		if !known[k] {
			p := k
			if path != "" {
				p = path + "." + k
			}
			*diags = append(*diags, LintDiag{
				Path:    p,
				Code:    "UNKNOWN_FIELD",
				Message: fmt.Sprintf("unknown field %q; possible AI hallucination or schema drift", k),
			})
		}
	}
}
