package game

import (
	"encoding/json"
	"fmt"
)

// LintJSON validates a raw JSON game document.
// It checks for unknown fields at the structural level (GameDoc, SettingsDoc,
// StructureDoc, GroupDoc, LocationDoc) that are silently dropped by the JSON
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
		"must_check_out": true, "show_team_count": true, "enable_points": true,
		"enable_bonus_points": true, "show_leaderboard": true,
	}
	knownStructureDocFields = map[string]bool{
		"routing": true, "completion": true, "minimum_required": true, "children": true,
	}
	knownGroupDocFields = map[string]bool{
		"id": true, "name": true, "color": true, "routing": true,
		"completion": true, "minimum_required": true, "auto_advance": true,
		"when": true, "children": true,
	}
	knownLocationDocFields = map[string]bool{
		"id": true, "slug": true, "name": true, "points": true,
		"when": true, "marker": true, "content": true, "navigation": true,
	}
	knownMarkerDocFields = map[string]bool{"lat": true, "lng": true}
	knownChildDocFields  = map[string]bool{"location": true, "group": true}
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
	if loc, ok := m["location"].(map[string]any); ok {
		locPath := path + ".location"
		warnUnknown(locPath, loc, knownLocationDocFields, diags)
		if marker, ok := loc["marker"].(map[string]any); ok {
			warnUnknown(locPath+".marker", marker, knownMarkerDocFields, diags)
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
