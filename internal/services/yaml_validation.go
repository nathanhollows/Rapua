package services

import (
	"fmt"
	"slices"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/models"
)

// ValidationError represents a fatal validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationWarning represents a non-fatal validation warning.
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult holds the result of validating a QuestDef.
type ValidationResult struct {
	Valid    bool                `json:"valid"`
	Errors   []ValidationError  `json:"errors,omitempty"`
	Warnings []ValidationWarning `json:"warnings,omitempty"`
}

// ValidateQuestDef validates a QuestDef and returns validation results.
func ValidateQuestDef(def *models.QuestDef) ValidationResult {
	result := ValidationResult{Valid: true}

	// Rule 1: version required, must be 1
	if def.Version == 0 {
		result.addError("version", "version is required")
	} else if def.Version != 1 {
		result.addError("version", fmt.Sprintf("unsupported version %d, must be 1", def.Version))
	}

	// Rule 2: name required
	if def.Name == "" {
		result.addError("name", "name is required")
	}

	// Rule 3: stop slugs unique
	slugs := make(map[string]bool)
	for _, stop := range def.Stops {
		if stop.Slug == "" {
			result.addError("stops", fmt.Sprintf("stop %q has no slug", stop.Name))
			continue
		}
		if slugs[stop.Slug] {
			result.addError("stops", fmt.Sprintf("duplicate stop slug %q", stop.Slug))
		}
		slugs[stop.Slug] = true
	}

	// Rule 4: every slug in stages.stops exists in stops list
	validateStageStops(&result, def.Structure.Stages, slugs)

	// Rule 5 & 6: block types registered and allowed in context
	blockIDs := make(map[string]bool)

	for _, stop := range def.Stops {
		validateBlocks(&result, stop.Content, blocks.ContextLocationContent, fmt.Sprintf("stops[%s].content", stop.Slug), blockIDs)
		validateBlocks(&result, stop.Clues, blocks.ContextLocationClues, fmt.Sprintf("stops[%s].clues", stop.Slug), blockIDs)
		validateBlocks(&result, stop.Tasks, blocks.ContextTask, fmt.Sprintf("stops[%s].tasks", stop.Slug), blockIDs)
		validateBlocks(&result, stop.Checkpoint, blocks.ContextCheckpoint, fmt.Sprintf("stops[%s].checkpoint", stop.Slug), blockIDs)
	}
	validateBlocks(&result, def.Start, blocks.ContextStart, "start", blockIDs)
	validateBlocks(&result, def.Finish, blocks.ContextFinish, "finish", blockIDs)

	// Rule 7: routing/navigation/completion are valid enum values
	validateStageEnums(&result, def.Structure.Stages)

	// Rule 8: stages must have name; minimum_required only when completion: minimum
	validateStageRules(&result, def.Structure.Stages)

	// Rule 10: no stage named "Unassigned"
	validateStageNames(&result, def.Structure.Stages)

	return result
}

func (r *ValidationResult) addError(field, message string) {
	r.Valid = false
	r.Errors = append(r.Errors, ValidationError{Field: field, Message: message})
}

func (r *ValidationResult) addWarning(field, message string) {
	r.Warnings = append(r.Warnings, ValidationWarning{Field: field, Message: message})
}

func validateStageStops(result *ValidationResult, stages []models.StageDef, slugs map[string]bool) {
	for _, stage := range stages {
		for _, slug := range stage.Stops {
			if !slugs[slug] {
				result.addError("structure.stages", fmt.Sprintf("stage %q references unknown stop slug %q", stage.Name, slug))
			}
		}
		validateStageStops(result, stage.Stages, slugs)
	}
}

func validateBlocks(result *ValidationResult, blockDefs []models.BlockDef, ctx blocks.BlockContext, path string, blockIDs map[string]bool) {
	for i, bd := range blockDefs {
		// Rule 5: block type registered
		if !blocks.CanBlockBeUsedInContext(bd.Type, ctx) {
			// Check if type exists at all
			if !blockTypeExists(bd.Type) {
				result.addError(fmt.Sprintf("%s[%d]", path, i), fmt.Sprintf("unknown block type %q", bd.Type))
			} else {
				// Rule 6: block type not allowed in this context
				result.addError(fmt.Sprintf("%s[%d]", path, i), fmt.Sprintf("block type %q not allowed in %s context", bd.Type, ctx))
			}
		}

		// Rule 9: block IDs unique
		if bd.ID != "" {
			if blockIDs[bd.ID] {
				result.addError(fmt.Sprintf("%s[%d]", path, i), fmt.Sprintf("duplicate block id %q", bd.ID))
			}
			blockIDs[bd.ID] = true
		}
	}
}

func blockTypeExists(blockType string) bool {
	// Check all contexts
	contexts := []blocks.BlockContext{
		blocks.ContextLocationContent,
		blocks.ContextLocationClues,
		blocks.ContextTask,
		blocks.ContextCheckpoint,
		blocks.ContextStart,
		blocks.ContextFinish,
	}
	for _, ctx := range contexts {
		if blocks.CanBlockBeUsedInContext(blockType, ctx) {
			return true
		}
	}
	return false
}

var validRouting = []string{"ordered", "randomised", "free_roam", "secret"}
var validNavigation = []string{"map", "labelled_map", "location_list", "custom", "tasks"}
var validCompletion = []string{"all", "minimum"}

func validateStageEnums(result *ValidationResult, stages []models.StageDef) {
	for _, stage := range stages {
		if stage.Routing != "" && !slices.Contains(validRouting, stage.Routing) {
			result.addError("structure.stages", fmt.Sprintf("stage %q has invalid routing %q", stage.Name, stage.Routing))
		}
		if stage.Navigation != "" && !slices.Contains(validNavigation, stage.Navigation) {
			result.addError("structure.stages", fmt.Sprintf("stage %q has invalid navigation %q", stage.Name, stage.Navigation))
		}
		if stage.Completion != "" && !slices.Contains(validCompletion, stage.Completion) {
			result.addError("structure.stages", fmt.Sprintf("stage %q has invalid completion %q", stage.Name, stage.Completion))
		}
		validateStageEnums(result, stage.Stages)
	}
}

func validateStageRules(result *ValidationResult, stages []models.StageDef) {
	for _, stage := range stages {
		if stage.Name == "" {
			result.addError("structure.stages", "stage must have a name")
		}
		if stage.MinimumRequired > 0 && stage.Completion != "minimum" {
			result.addWarning("structure.stages", fmt.Sprintf("stage %q has minimum_required but completion is not 'minimum'", stage.Name))
		}
		validateStageRules(result, stage.Stages)
	}
}

func validateStageNames(result *ValidationResult, stages []models.StageDef) {
	for _, stage := range stages {
		if stage.Name == "Unassigned" {
			result.addWarning("structure.stages", "'Unassigned' stops will be placed at root level (not in a named stage)")
		}
		validateStageNames(result, stage.Stages)
	}
}
