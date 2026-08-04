package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nathanhollows/Rapua/v7/game"
)

// ChoiceOption is a single selectable option in a choice block.
type ChoiceOption struct {
	Label string `json:"label"`
	// Sets is the variable name set to "true" when this option is chosen.
	Sets string `json:"sets"`
}

// ChoiceBlock presents a set of labelled options. In single-select mode the player
// picks exactly one; in multi-select mode they may pick any number. Each chosen
// option's Sets variable is written as "true". Choice is final — re-submission
// is a no-op because variables are monotonic.
type ChoiceBlock struct {
	BaseBlock
	Prompt      string         `json:"prompt"`
	ButtonText  string         `json:"button_text,omitempty"`
	MultiSelect bool           `json:"multi_select,omitempty"`
	Options     []ChoiceOption `json:"options"`
}

type choicePlayerData struct {
	Chosen []string `json:"chosen"` // Sets var names of all chosen options
}

// NewChoiceBlock creates a new ChoiceBlock from a BaseBlock.
func NewChoiceBlock(base BaseBlock) *ChoiceBlock {
	return &ChoiceBlock{BaseBlock: base}
}

// Basic attribute getters

func (b *ChoiceBlock) GetID() string      { return b.ID }
func (b *ChoiceBlock) GetType() string    { return "choice" }
func (b *ChoiceBlock) GetOwnerID() string { return b.OwnerID }
func (b *ChoiceBlock) GetName() string    { return "Choice" }
func (b *ChoiceBlock) GetDescription() string {
	return "Presents labelled options; selecting one or more sets boolean variables."
}
func (b *ChoiceBlock) GetOrder() int  { return b.Order }
func (b *ChoiceBlock) GetPoints() int { return b.Points }
func (b *ChoiceBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-list-icon lucide-list"><path d="M3 12h.01"/><path d="M3 18h.01"/><path d="M3 6h.01"/><path d="M8 12h13"/><path d="M8 18h13"/><path d="M8 6h13"/></svg>`
}

func (b *ChoiceBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data operations

func (b *ChoiceBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *ChoiceBlock) UpdateBlockData(input map[string][]string) error {
	if pts, ok := input["points"]; ok && len(pts) > 0 {
		p, err := strconv.Atoi(pts[0])
		if err != nil {
			return errors.New("points must be an integer")
		}
		b.Points = p
	}
	if prompt, ok := input["prompt"]; ok && len(prompt) > 0 {
		b.Prompt = prompt[0]
	}
	if bt, ok := input["button_text"]; ok && len(bt) > 0 {
		b.ButtonText = bt[0]
	}
	b.MultiSelect = len(input["multi_select"]) > 0 && input["multi_select"][0] == "on"
	// Rebuild options from parallel label[]/sets[] slices sent by the admin form.
	labels := input["option_label"]
	sets := input["option_sets"]
	if labels != nil || sets != nil {
		count := len(labels)
		if len(sets) < count {
			count = len(sets)
		}
		opts := make([]ChoiceOption, 0, count)
		for i := range count {
			if labels[i] == "" && sets[i] == "" {
				continue
			}
			opts = append(opts, ChoiceOption{Label: labels[i], Sets: sets[i]})
		}
		b.Options = opts
	}
	return nil
}

// ToYAML returns the block's fields for YAML export.
func (b *ChoiceBlock) ToYAML() map[string]any {
	opts := make([]map[string]any, 0, len(b.Options))
	for _, o := range b.Options {
		opts = append(opts, map[string]any{
			"label": o.Label,
			"sets":  o.Sets,
		})
	}
	m := map[string]any{
		"options": opts,
	}
	if b.Prompt != "" {
		m["prompt"] = b.Prompt
	}
	if b.ButtonText != "" {
		m["button_text"] = b.ButtonText
	}
	if b.MultiSelect {
		m["multi_select"] = true
	}
	return m
}

// Validation

func (b *ChoiceBlock) RequiresValidation() bool   { return true }
func (b *ChoiceBlock) SupportsVariableSets() bool { return true }

// ValidatePlayerInput records the player's choice(s). Re-submission after completion
// is a no-op — choice is final and variables are monotonic.
func (b *ChoiceBlock) ValidatePlayerInput(
	state PlayerState,
	input map[string][]string,
) (PlayerState, error) {
	if state.IsComplete() {
		return state, nil
	}

	submitted := input["choice"]
	if len(submitted) == 0 || (len(submitted) == 1 && submitted[0] == "") {
		return state, errors.New("no choice submitted")
	}

	// Build a set of valid option var names for fast lookup.
	validOpts := make(map[string]struct{}, len(b.Options))
	for _, opt := range b.Options {
		if opt.Sets != "" {
			validOpts[opt.Sets] = struct{}{}
		}
	}

	var chosen []string
	seen := make(map[string]struct{})
	for _, c := range submitted {
		if _, ok := validOpts[c]; !ok {
			return state, fmt.Errorf("invalid choice %q", c)
		}
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		chosen = append(chosen, c)
		if !b.MultiSelect {
			break // single-select: accept only the first valid value
		}
	}

	if len(chosen) == 0 {
		return state, errors.New("no valid choice submitted")
	}

	data, err := json.Marshal(choicePlayerData{Chosen: chosen})
	if err != nil {
		return state, errors.New("error saving player data")
	}
	state.SetPlayerData(data)
	state.SetComplete(true)
	state.SetPointsAwarded(b.Points)
	return state, nil
}

// GetSets overrides BaseBlock to return all option var names.
// Var-writing is handled exclusively by GetTriggeredVars (ChoiceVarSetter).
// This method exists for: (1) admin variables endpoint listing, (2) UNUSED_VAR lint.
func (b *ChoiceBlock) GetSets() game.SetsField {
	vars := make(game.SetsField, len(b.Options))
	for _, opt := range b.Options {
		if opt.Sets != "" {
			vars[opt.Sets] = "true"
		}
	}
	return vars
}

// GetTriggeredVars implements ChoiceVarSetter. Returns each chosen option's var
// as "true", or nil if the block is not yet complete.
func (b *ChoiceBlock) GetTriggeredVars(state PlayerState) map[string]string {
	if !state.IsComplete() {
		return nil
	}
	vars := b.GetChosenVars(state)
	if len(vars) == 0 {
		return nil
	}
	result := make(map[string]string, len(vars))
	for _, v := range vars {
		result[v] = "true"
	}
	return result
}

// GetChosenVars extracts all chosen option var names from state.
func (b *ChoiceBlock) GetChosenVars(state PlayerState) []string {
	if state == nil || state.GetPlayerData() == nil {
		return nil
	}
	var data choicePlayerData
	if err := json.Unmarshal(state.GetPlayerData(), &data); err != nil {
		return nil
	}
	return data.Chosen
}

// GetButtonText returns the submit button label, falling back to "Confirm choice".
func (b *ChoiceBlock) GetButtonText() string {
	if b.ButtonText != "" {
		return b.ButtonText
	}
	return "Confirm choice"
}

// GetChosenLabels returns the display labels for all chosen options.
func (b *ChoiceBlock) GetChosenLabels(state PlayerState) []string {
	chosen := b.GetChosenVars(state)
	labels := make([]string, 0, len(chosen))
	for _, varName := range chosen {
		for _, opt := range b.Options {
			if opt.Sets == varName {
				labels = append(labels, opt.Label)
				break
			}
		}
	}
	return labels
}

// GetChosenLabel returns all chosen option labels as a comma-joined string.
func (b *ChoiceBlock) GetChosenLabel(state PlayerState) string {
	return strings.Join(b.GetChosenLabels(state), ", ")
}

// DocSetsVars implements game.BlockDocVarsProvider. Extracts var names from
// options[*].sets in the raw block doc (used by the linter before ParseData runs).
func (b *ChoiceBlock) DocSetsVars(doc game.BlockDoc) []string {
	opts, ok := doc["options"].([]any)
	if !ok {
		return nil
	}
	var vars []string
	for _, opt := range opts {
		m, ok := opt.(map[string]any)
		if !ok {
			continue
		}
		if s, ok := m["sets"].(string); ok && s != "" {
			vars = append(vars, s)
		}
	}
	return vars
}

// ValidateBlockDoc implements game.BlockDocValidator. Returns structural lint
// diagnostics specific to choice blocks.
func (b *ChoiceBlock) ValidateBlockDoc(path string, doc game.BlockDoc) ([]game.LintDiag, []game.LintDiag) {
	var errs, warns []game.LintDiag
	raw, hasOpts := doc["options"]
	if !hasOpts || raw == nil {
		errs = append(errs, game.LintDiag{
			Path:    path + ".options",
			Code:    "CHOICE_NO_OPTIONS",
			Message: "choice block must have at least one option",
		})
		return errs, warns
	}
	opts, ok := raw.([]any)
	if !ok || len(opts) == 0 {
		errs = append(errs, game.LintDiag{
			Path:    path + ".options",
			Code:    "CHOICE_NO_OPTIONS",
			Message: "choice block must have at least one option",
		})
		return errs, warns
	}
	for i, opt := range opts {
		m, ok := opt.(map[string]any)
		if !ok {
			continue
		}
		optPath := fmt.Sprintf("%s.options[%d]", path, i)
		s, _ := m["sets"].(string)
		if s == "" {
			errs = append(errs, game.LintDiag{
				Path:    optPath + ".sets",
				Code:    "CHOICE_OPTION_MISSING_SETS",
				Message: "choice option must have a non-empty \"sets\" variable name",
			})
		}
		label, _ := m["label"].(string)
		if label == "" {
			warns = append(warns, game.LintDiag{
				Path:    optPath + ".label",
				Code:    "CHOICE_OPTION_MISSING_LABEL",
				Message: "choice option has no label; players will see an empty option",
			})
		}
	}
	return errs, warns
}
