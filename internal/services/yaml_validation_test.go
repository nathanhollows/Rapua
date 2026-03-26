package services_test

import (
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v7/internal/services"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validQuestDef() *models.QuestDef {
	return &models.QuestDef{
		Version: 1,
		Name:    "Test Quest",
		Stops: []models.StopDef{
			{Slug: "stop-one", Name: "Stop One"},
			{Slug: "stop-two", Name: "Stop Two"},
		},
	}
}

func TestValidateQuestDef_Valid(t *testing.T) {
	result := services.ValidateQuestDef(validQuestDef())
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

func TestValidateQuestDef_MissingVersion(t *testing.T) {
	def := validQuestDef()
	def.Version = 0
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "version", result.Errors[0].Field)
}

func TestValidateQuestDef_UnsupportedVersion(t *testing.T) {
	def := validQuestDef()
	def.Version = 99
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
	assert.True(t, strings.Contains(result.Errors[0].Message, "unsupported"))
}

func TestValidateQuestDef_MissingName(t *testing.T) {
	def := validQuestDef()
	def.Name = ""
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
	hasNameError := false
	for _, e := range result.Errors {
		if e.Field == "name" {
			hasNameError = true
		}
	}
	assert.True(t, hasNameError, "expected a 'name' error")
}

func TestValidateQuestDef_DuplicateStopSlugs(t *testing.T) {
	def := validQuestDef()
	def.Stops = append(def.Stops, models.StopDef{Slug: "stop-one", Name: "Duplicate"})
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
	hasDuplicate := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "duplicate") {
			hasDuplicate = true
		}
	}
	assert.True(t, hasDuplicate, "expected duplicate slug error")
}

func TestValidateQuestDef_StopMissingSlug(t *testing.T) {
	def := validQuestDef()
	def.Stops = append(def.Stops, models.StopDef{Slug: "", Name: "No Slug"})
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
}

func TestValidateQuestDef_StageReferencesUnknownSlug(t *testing.T) {
	def := validQuestDef()
	def.Structure.Stages = []models.StageDef{
		{Name: "Stage 1", Stops: []string{"nonexistent-slug"}},
	}
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
}

func TestValidateQuestDef_StageReferencesKnownSlug(t *testing.T) {
	def := validQuestDef()
	def.Structure.Stages = []models.StageDef{
		{Name: "Stage 1", Stops: []string{"stop-one", "stop-two"}},
	}
	result := services.ValidateQuestDef(def)
	assert.True(t, result.Valid)
}

func TestValidateQuestDef_InvalidBlockType(t *testing.T) {
	def := validQuestDef()
	def.Stops[0].Content = []models.BlockDef{
		{Type: "totally_nonexistent_block_xyz"},
	}
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
}

func TestValidateQuestDef_ValidBlockType(t *testing.T) {
	def := validQuestDef()
	def.Stops[0].Content = []models.BlockDef{
		{Type: "text"},
	}
	result := services.ValidateQuestDef(def)
	assert.True(t, result.Valid, "errors: %v", result.Errors)
}

func TestValidateQuestDef_InvalidStageRouting(t *testing.T) {
	def := validQuestDef()
	def.Structure.Stages = []models.StageDef{
		{Name: "Stage 1", Stops: []string{"stop-one"}, Routing: "invalid_routing"},
	}
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
}

func TestValidateQuestDef_ValidStageEnums(t *testing.T) {
	def := validQuestDef()
	def.Structure.Stages = []models.StageDef{
		{
			Name:       "Stage 1",
			Stops:      []string{"stop-one"},
			Routing:    "free_roam",
			Navigation: "map",
			Completion: "all",
		},
	}
	result := services.ValidateQuestDef(def)
	assert.True(t, result.Valid, "errors: %v", result.Errors)
}

func TestValidateQuestDef_StageWithoutName(t *testing.T) {
	def := validQuestDef()
	def.Structure.Stages = []models.StageDef{
		{Name: "", Stops: []string{"stop-one"}},
	}
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
}

func TestValidateQuestDef_DuplicateBlockIDs(t *testing.T) {
	def := validQuestDef()
	def.Stops[0].Content = []models.BlockDef{
		{Type: "text", ID: "block-123"},
	}
	def.Stops[1].Content = []models.BlockDef{
		{Type: "text", ID: "block-123"},
	}
	result := services.ValidateQuestDef(def)
	assert.False(t, result.Valid)
	hasDuplicate := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "duplicate block id") {
			hasDuplicate = true
		}
	}
	assert.True(t, hasDuplicate, "expected duplicate block ID error")
}

func TestValidateQuestDef_UnassignedStageWarning(t *testing.T) {
	def := validQuestDef()
	def.Structure.Stages = []models.StageDef{
		{Name: "Unassigned", Stops: []string{"stop-one"}},
	}
	result := services.ValidateQuestDef(def)
	// "Unassigned" produces a warning, not an error
	assert.True(t, result.Valid, "errors: %v", result.Errors)
	hasWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "Unassigned") {
			hasWarning = true
		}
	}
	assert.True(t, hasWarning, "expected Unassigned warning")
}

func TestValidateQuestDef_MinimumRequiredWithoutMinimumCompletion(t *testing.T) {
	def := validQuestDef()
	def.Structure.Stages = []models.StageDef{
		{Name: "Stage 1", Stops: []string{"stop-one"}, MinimumRequired: 1, Completion: "all"},
	}
	result := services.ValidateQuestDef(def)
	// Should produce a warning, not an error
	assert.True(t, result.Valid, "errors: %v", result.Errors)
	hasWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "minimum_required") {
			hasWarning = true
		}
	}
	assert.True(t, hasWarning, "expected minimum_required warning")
}
