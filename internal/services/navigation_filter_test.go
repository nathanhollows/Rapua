package services //nolint:testpackage // tests unexported filterGameStructure and filterLocationsByWhen

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// staticResolver resolves a fixed set of vars (for testing).
type staticResolver map[string]string

func (s staticResolver) ResolveVar(name string) (string, bool) {
	v, ok := s[name]
	return v, ok
}

func TestFilterGameStructure_HidesGroup(t *testing.T) {
	when := &game.WhenClause{AllOf: []game.Condition{{Var: "gate"}}}
	gs := models.GameStructure{
		ID: "root",
		SubGroups: []models.GameStructure{
			{ID: "visible", When: nil},
			{ID: "hidden", When: when},
		},
	}

	// gate unset → hidden group removed
	resolver := staticResolver{}
	got := filterGameStructure(gs, resolver, nil)
	assert.Len(t, got.SubGroups, 1)
	assert.Equal(t, "visible", got.SubGroups[0].ID)
}

func TestFilterGameStructure_ShowsGroupWhenConditionMet(t *testing.T) {
	when := &game.WhenClause{AllOf: []game.Condition{{Var: "gate"}}}
	gs := models.GameStructure{
		ID: "root",
		SubGroups: []models.GameStructure{
			{ID: "conditional", When: when},
		},
	}

	resolver := staticResolver{"gate": "true"}
	got := filterGameStructure(gs, resolver, nil)
	assert.Len(t, got.SubGroups, 1)
	assert.Equal(t, "conditional", got.SubGroups[0].ID)
}

func TestFilterGameStructure_Recursive(t *testing.T) {
	when := &game.WhenClause{AllOf: []game.Condition{{Var: "deep"}}}
	gs := models.GameStructure{
		ID: "root",
		SubGroups: []models.GameStructure{
			{
				ID: "outer",
				SubGroups: []models.GameStructure{
					{ID: "inner-visible", When: nil},
					{ID: "inner-hidden", When: when},
				},
			},
		},
	}

	resolver := staticResolver{}
	got := filterGameStructure(gs, resolver, nil)
	assert.Len(t, got.SubGroups, 1)
	assert.Equal(t, "outer", got.SubGroups[0].ID)
	assert.Len(t, got.SubGroups[0].SubGroups, 1)
	assert.Equal(t, "inner-visible", got.SubGroups[0].SubGroups[0].ID)
}

func TestFilterGameStructure_RemovesHiddenLocationIDs(t *testing.T) {
	gs := models.GameStructure{
		ID:          "root",
		LocationIDs: []string{"loc-a", "loc-b", "loc-c"},
	}

	hidden := map[string]bool{"loc-b": true}
	got := filterGameStructure(gs, staticResolver{}, hidden)
	assert.Equal(t, []string{"loc-a", "loc-c"}, got.LocationIDs)
}

func TestFilterGameStructure_RemovesHiddenLocationIDsInSubGroups(t *testing.T) {
	gs := models.GameStructure{
		ID: "root",
		SubGroups: []models.GameStructure{
			{ID: "g1", LocationIDs: []string{"loc-x", "loc-y"}},
		},
	}

	hidden := map[string]bool{"loc-x": true}
	got := filterGameStructure(gs, staticResolver{}, hidden)
	assert.Len(t, got.SubGroups, 1)
	assert.Equal(t, []string{"loc-y"}, got.SubGroups[0].LocationIDs)
}

func TestFilterLocationsByWhen_FiltersHidden(t *testing.T) {
	when := &game.WhenClause{AllOf: []game.Condition{{Var: "flag"}}}
	locs := []models.Location{
		{Slug: "open", When: nil},
		{Slug: "gated", When: when},
	}

	resolver := staticResolver{}
	got := filterLocationsByWhen(locs, resolver)
	assert.Len(t, got, 1)
	assert.Equal(t, "open", got[0].Slug)
}

func TestFilterLocationsByWhen_ShowsWhenConditionMet(t *testing.T) {
	when := &game.WhenClause{AllOf: []game.Condition{{Var: "flag"}}}
	locs := []models.Location{
		{Slug: "gated", When: when},
	}

	resolver := staticResolver{"flag": "true"}
	got := filterLocationsByWhen(locs, resolver)
	assert.Len(t, got, 1)
}

func TestFilterLocationsByWhen_EmptySlice(t *testing.T) {
	got := filterLocationsByWhen([]models.Location{}, staticResolver{})
	assert.Empty(t, got)
}
