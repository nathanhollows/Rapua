package game

import (
	"encoding/json"
	"fmt"
)

// LintJSON validates a raw JSON game document.
// It checks for unknown fields at the structural level (GameDoc, SettingsDoc,
// StructureDoc, GroupDoc, ObjectiveDoc) that are silently dropped by the JSON
// parser, then runs the full Lint pass on the parsed document.
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
	knownStructureDocFields = map[string]bool{
		"routing": true, "completion": true, "minimum_required": true, "children": true,
	}
	knownGroupDocFields = map[string]bool{
		"id": true, "name": true, "color": true, "routing": true,
		"completion": true, "minimum_required": true, "auto_advance": true,
		"when": true, "children": true,
	}
	knownObjectiveDocFields = map[string]bool{
		"id": true, "slug": true, "title": true, "when": true,
		"proof": true, "reveal": true,
	}
	knownObjectiveContextDocFields = map[string]bool{"blocks": true, "sets": true}
	knownChildDocFields            = map[string]bool{"group": true, "objective": true}
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
		warnUnknown("structure", st, knownStructureDocFields, &diags)
		if children, ok := st["children"].([]any); ok {
			for i, child := range children {
				checkUnknownChild(fmt.Sprintf("structure.children[%d]", i), child, &diags)
			}
		}
	}
	return diags
}

func checkUnknownChild(path string, child any, diags *[]LintDiag) {
	m, ok := child.(map[string]any)
	if !ok {
		return
	}
	warnUnknown(path, m, knownChildDocFields, diags)
	if obj, ok := m["objective"].(map[string]any); ok {
		objPath := path + ".objective"
		warnUnknown(objPath, obj, knownObjectiveDocFields, diags)
		if proof, ok := obj["proof"].(map[string]any); ok {
			warnUnknown(objPath+".proof", proof, knownObjectiveContextDocFields, diags)
		}
		if reveal, ok := obj["reveal"].(map[string]any); ok {
			warnUnknown(objPath+".reveal", reveal, knownObjectiveContextDocFields, diags)
		}
	}
	if grp, ok := m["group"].(map[string]any); ok {
		grpPath := path + ".group"
		warnUnknown(grpPath, grp, knownGroupDocFields, diags)
		if children, ok := grp["children"].([]any); ok {
			for i, child := range children {
				checkUnknownChild(fmt.Sprintf("%s.children[%d]", grpPath, i), child, diags)
			}
		}
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
