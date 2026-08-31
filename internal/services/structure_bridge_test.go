package services //nolint:testpackage // testing the unexported storage bridge

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func intPtr(n int) *int { return &n }

func TestBandFromGroup(t *testing.T) {
	tests := []struct {
		name            string
		group           models.GameStructure
		wantMinChildren *int
		wantMaxChildren *int
	}{
		{
			name:  "completion all omits both bounds",
			group: models.GameStructure{CompletionType: models.CompletionAll},
		},
		{
			name: "minimum with auto-advance auto-completes at the minimum",
			group: models.GameStructure{
				CompletionType: models.CompletionMinimum, MinimumRequired: 1, AutoAdvance: true,
			},
			wantMinChildren: intPtr(1), wantMaxChildren: intPtr(1),
		},
		{
			name: "minimum without auto-advance leaves the player to finish",
			group: models.GameStructure{
				CompletionType: models.CompletionMinimum, MinimumRequired: 5, AutoAdvance: false,
			},
			wantMinChildren: intPtr(5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := bandFromGroup(tt.group)
			assert.Equal(t, tt.wantMinChildren, gotMin)
			assert.Equal(t, tt.wantMaxChildren, gotMax)
		})
	}
}

func TestGroupCompletionFromBand(t *testing.T) {
	threeChildren := []game.ObjectiveDoc{{}, {}, {}}

	tests := []struct {
		name            string
		obj             game.ObjectiveDoc
		wantCompletion  models.CompletionType
		wantMinRequired int
		wantAutoAdvance bool
	}{
		{
			name:            "omitted band is completion all",
			obj:             game.ObjectiveDoc{Children: threeChildren},
			wantCompletion:  models.CompletionAll,
			wantAutoAdvance: true,
		},
		{
			name: "min equal to max auto-advances at that count",
			obj: game.ObjectiveDoc{
				Children: threeChildren, ChildrenMin: intPtr(1), ChildrenMax: intPtr(1),
			},
			wantCompletion:  models.CompletionMinimum,
			wantMinRequired: 1,
			wantAutoAdvance: true,
		},
		{
			name:            "min only leaves the player to finish",
			obj:             game.ObjectiveDoc{Children: threeChildren, ChildrenMin: intPtr(2)},
			wantCompletion:  models.CompletionMinimum,
			wantMinRequired: 2,
		},
		{
			name:            "an explicit zero min is a band, not completion all",
			obj:             game.ObjectiveDoc{Children: threeChildren, ChildrenMin: intPtr(0)},
			wantCompletion:  models.CompletionMinimum,
			wantMinRequired: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completion, minRequired, autoAdvance := groupCompletionFromBand(tt.obj)
			assert.Equal(t, tt.wantCompletion, completion)
			assert.Equal(t, tt.wantMinRequired, minRequired)
			assert.Equal(t, tt.wantAutoAdvance, autoAdvance)
		})
	}
}

// The engine's trio cannot express a range, so it narrows to the minimum. The
// band itself is stored alongside, which is what keeps the round trip lossless.
func TestGroupCompletionFromBand_RangeNarrowsToItsMinimum(t *testing.T) {
	obj := game.ObjectiveDoc{
		Children:    []game.ObjectiveDoc{{}, {}, {}, {}},
		ChildrenMin: intPtr(1),
		ChildrenMax: intPtr(3),
	}

	completion, minRequired, autoAdvance := groupCompletionFromBand(obj)
	assert.Equal(t, models.CompletionMinimum, completion)
	assert.Equal(t, 1, minRequired)
	assert.False(t, autoAdvance, "a range still waits on the player")
}

// Every band shape must survive storage byte-for-byte. Deriving it back from
// the engine's completion trio would silently rewrite what the author authored.
func TestBandSurvivesStorageExactly(t *testing.T) {
	tests := []struct {
		name        string
		minChildren *int
		maxChildren *int
	}{
		{name: "both omitted"},
		{name: "min equals max", minChildren: intPtr(1), maxChildren: intPtr(1)},
		{name: "a genuine range", minChildren: intPtr(1), maxChildren: intPtr(3)},
		{name: "min only", minChildren: intPtr(2)},
		{name: "max only", maxChildren: intPtr(2)},
		{name: "explicit zero min", minChildren: intPtr(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored := newSectionGroup("id", game.ObjectiveDoc{
				Slug: "wing", Title: "Wing",
				Children:    []game.ObjectiveDoc{{}, {}, {}, {}},
				ChildrenMin: tt.minChildren,
				ChildrenMax: tt.maxChildren,
			})

			gotMin, gotMax := bandFromGroup(stored)
			assert.Equal(t, tt.minChildren, gotMin)
			assert.Equal(t, tt.maxChildren, gotMax)
		})
	}
}

// A group written before the band existed carries only the completion trio, so
// the band is read back from that rather than coming out empty.
func TestBandFromGroup_FallsBackToTheCompletionTrio(t *testing.T) {
	legacy := models.GameStructure{
		CompletionType: models.CompletionMinimum, MinimumRequired: 2, AutoAdvance: true,
	}
	gotMin, gotMax := bandFromGroup(legacy)
	assert.Equal(t, intPtr(2), gotMin)
	assert.Equal(t, intPtr(2), gotMax)
}

// A group may legally have a blank name, but an objective may not have a blank
// title. Exporting one verbatim would emit a document that fails its own import.
func TestSectionTitle_NeverBlank(t *testing.T) {
	assert.Equal(t, "Wave 1", sectionTitle(models.GameStructure{Name: "Wave 1"}))
	assert.NotEmpty(t, sectionTitle(models.GameStructure{}))
}

// A stored slug is reused rather than re-minted, so renaming a section does not
// silently break a depends list that names it.
func TestSectionSlug_PrefersTheStoredSlug(t *testing.T) {
	assert.Equal(t, "kept", sectionSlug(models.GameStructure{Slug: "kept", Name: "Renamed Since"}))
	assert.Equal(t, "wave-1", sectionSlug(models.GameStructure{Name: "Wave 1"}))
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "Wave 1", want: "wave-1"},
		{name: "The Perfumer's Garden", want: "the-perfumer-s-garden"},
		{name: "  Leading and trailing  ", want: "leading-and-trailing"},
		{name: "ALL CAPS", want: "all-caps"},
		{name: "!!!", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, slugify(tt.name))
		})
	}
}

func TestUniqueSlug(t *testing.T) {
	taken := map[string]bool{"lobby": true}

	assert.Equal(t, "wave-1", uniqueSlug("wave-1", taken))
	assert.Equal(t, "wave-1-2", uniqueSlug("wave-1", taken), "a repeat name gets a suffix")
	assert.Equal(t, "wave-1-3", uniqueSlug("wave-1", taken))
	assert.Equal(t, "lobby-2", uniqueSlug("lobby", taken), "objective slugs are claimed first")
	assert.Equal(t, "section", uniqueSlug("", taken), "an unsluggable name still needs a slug")
}
