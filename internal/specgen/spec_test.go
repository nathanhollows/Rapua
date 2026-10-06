package specgen_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/specgen"
)

// TestSpecStaleness ensures every JSON-tagged field on each block struct has a
// corresponding FieldSpec entry in its GetSpec(). If someone adds a field to a
// block but forgets to update GetSpec(), this test fails.
func TestSpecStaleness(t *testing.T) {
	registered := blocks.GetRegisteredBlocks()

	for _, reg := range registered {
		sp, ok := reg.Prototype.(game.SpecProvider)
		if !ok {
			t.Errorf("block type %q does not implement game.SpecProvider", reg.BlockType)
			continue
		}

		spec := sp.GetSpec()
		if spec.Type == "" {
			t.Errorf("block type %q: GetSpec().Type is empty", reg.BlockType)
		}
		if spec.Type != reg.BlockType {
			t.Errorf("block type %q: GetSpec().Type = %q, want %q", reg.BlockType, spec.Type, reg.BlockType)
		}

		// Collect all field names from the spec (flattened to top-level only)
		specFields := make(map[string]bool, len(spec.Fields))
		for _, f := range spec.Fields {
			specFields[f.Name] = true
		}

		// Reflect-walk the block struct's exported JSON-tagged fields
		blockType := reflect.TypeOf(reg.Prototype)
		if blockType.Kind() == reflect.Pointer {
			blockType = blockType.Elem()
		}

		for i := range blockType.NumField() {
			field := blockType.Field(i)

			// Skip embedded BaseBlock fields (they're promoted, not block-specific)
			if field.Anonymous {
				continue
			}

			tag := field.Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}

			// Extract the field name (before any comma options like omitempty)
			jsonName := strings.Split(tag, ",")[0]
			if jsonName == "-" || jsonName == "" {
				continue
			}

			if !specFields[jsonName] {
				t.Errorf("block type %q: field %q (json:%q) exists in struct but is missing from GetSpec().Fields",
					reg.BlockType, field.Name, jsonName)
			}
		}
	}
}

// TestGenerateBlockSpecs verifies that all registered blocks produce specs.
func TestGenerateBlockSpecs(t *testing.T) {
	specs := specgen.GenerateBlockSpecs()

	registeredCount := len(blocks.GetRegisteredBlocks())
	if len(specs) != registeredCount {
		t.Errorf("GenerateBlockSpecs() returned %d specs, want %d (one per registered block)",
			len(specs), registeredCount)
	}

	seen := make(map[string]bool)
	for _, spec := range specs {
		if spec.Type == "" {
			t.Error("spec has empty Type")
		}
		if seen[spec.Type] {
			t.Errorf("duplicate spec type %q", spec.Type)
		}
		seen[spec.Type] = true
	}
}

// TestGenerateJSON ensures the spec serialises without error.
func TestGenerateJSON(t *testing.T) {
	data, err := specgen.GenerateJSON()
	if err != nil {
		t.Fatalf("GenerateJSON() error: %v", err)
	}
	if len(data) == 0 {
		t.Error("GenerateJSON() returned empty output")
	}
}

// TestGenerateFullSpec_Version checks that the generated spec reports version "v8".
func TestGenerateFullSpec_Version(t *testing.T) {
	spec := specgen.GenerateFullSpec()
	if spec.Version != "v8" {
		t.Errorf("GenerateFullSpec().Version = %q, want %q", spec.Version, "v8")
	}
}

// TestGenerateFullSpec_HasAllContexts checks all expected context values are present.
func TestGenerateFullSpec_HasAllContexts(t *testing.T) {
	spec := specgen.GenerateFullSpec()

	contextValues := make(map[string]bool, len(spec.Contexts))
	for _, c := range spec.Contexts {
		contextValues[c.Value] = true
	}

	expected := []string{
		"start", "finish", "objective_proof", "objective_reveal",
	}
	for _, ctx := range expected {
		if !contextValues[ctx] {
			t.Errorf("GenerateFullSpec().Contexts missing %q", ctx)
		}
	}
}

// TestGenerateBlockSpecs_ContextsMatchRegistry proves each block's generated
// Contexts comes from the live registry (blocks.block.go's registerBlock
// calls), not a hand-maintained copy that can drift out of sync with it.
func TestGenerateBlockSpecs_ContextsMatchRegistry(t *testing.T) {
	registered := blocks.GetRegisteredBlocks()
	byType := make(map[string][]game.BlockContext, len(registered))
	for _, reg := range registered {
		byType[reg.BlockType] = reg.SupportedContexts
	}

	for _, spec := range specgen.GenerateBlockSpecs() {
		want := byType[spec.Type]
		if len(spec.Contexts) != len(want) {
			t.Errorf("block %q: Contexts = %v, want %v", spec.Type, spec.Contexts, want)
			continue
		}
		for i, c := range want {
			if spec.Contexts[i] != string(c) {
				t.Errorf("block %q: Contexts[%d] = %q, want %q", spec.Type, i, spec.Contexts[i], string(c))
			}
		}
		for _, c := range spec.Contexts {
			if c == "location_content" || c == "navigation" {
				t.Errorf("block %q: Contexts still lists dead context %q", spec.Type, c)
			}
		}
	}
}

// TestGenerateFullSpec_HasEnums checks the routing enum is non-empty. Routing
// is the only enum left: the completion enum went with the band that replaced it.
func TestGenerateFullSpec_HasEnums(t *testing.T) {
	spec := specgen.GenerateFullSpec()
	if len(spec.Enums.Routing) == 0 {
		t.Error("GenerateFullSpec().Enums.Routing is empty")
	}
}

// TestGenerateBlockSpecs_Sorted verifies specs are returned in ascending type order.
func TestGenerateBlockSpecs_Sorted(t *testing.T) {
	specs := specgen.GenerateBlockSpecs()
	for i := 1; i < len(specs); i++ {
		if specs[i-1].Type > specs[i].Type {
			t.Errorf("specs not sorted: %q comes before %q", specs[i-1].Type, specs[i].Type)
		}
	}
}

// TestGenerateFullSpec_BlockCountMatchesRegistry verifies one spec per registered block.
func TestGenerateFullSpec_BlockCountMatchesRegistry(t *testing.T) {
	spec := specgen.GenerateFullSpec()
	want := len(blocks.GetRegisteredBlocks())
	if len(spec.Blocks) != want {
		t.Errorf("GenerateFullSpec().Blocks len = %d, want %d", len(spec.Blocks), want)
	}
}

// TestGenerateFullSpec_DocumentHasRequiredFields checks the document spec contains
// top-level required fields like "rapua", "name", "structure".
func TestGenerateFullSpec_DocumentHasRequiredFields(t *testing.T) {
	spec := specgen.GenerateFullSpec()

	fieldNames := make(map[string]bool, len(spec.Document.Fields))
	for _, f := range spec.Document.Fields {
		fieldNames[f.Name] = true
	}

	requiredFields := []string{"rapua", "name", "settings", "start", "finish", "structure"}
	for _, field := range requiredFields {
		if !fieldNames[field] {
			t.Errorf("document spec missing required field %q", field)
		}
	}
}

// TestGenerateFullSpec_SpecProviderUnused checks no registered blocks are missing GetSpec().
// Complements TestSpecStaleness by ensuring all blocks produce non-empty types.
func TestGenerateFullSpec_AllBlocksHaveNonEmptyType(t *testing.T) {
	specs := specgen.GenerateBlockSpecs()
	for _, s := range specs {
		if s.Type == "" {
			t.Error("GenerateBlockSpecs() returned a spec with empty Type")
		}
		if s.Name == "" {
			t.Errorf("spec for type %q has empty Name", s.Type)
		}
	}
}

func TestGenerateBlockSpecs_PointsOnlyOnInteractiveBlocks(t *testing.T) {
	registered := blocks.GetRegisteredBlocks()
	wantPoints := make(map[string]bool, len(registered))
	for _, reg := range registered {
		wantPoints[reg.BlockType] = reg.Prototype.RequiresValidation()
	}

	for _, spec := range specgen.GenerateBlockSpecs() {
		hasPoints := false
		for _, f := range spec.SharedFields {
			if f == "points" {
				hasPoints = true
			}
		}
		if want := wantPoints[spec.Type]; hasPoints != want {
			t.Errorf("block %q: SharedFields has points=%v, want %v (RequiresValidation)",
				spec.Type, hasPoints, want)
		}
	}
}

func jsonFieldNames(t reflect.Type) map[string]bool {
	names := make(map[string]bool, t.NumField())
	for i := range t.NumField() {
		field := t.Field(i)
		if field.Anonymous {
			continue
		}
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		names[name] = true
	}
	return names
}

// TestSpecStaleness_ObjectiveFields checks both directions because the spec's
// field list is hand-written and drifts when ObjectiveDoc changes.
func TestSpecStaleness_ObjectiveFields(t *testing.T) {
	spec := specgen.GenerateFullSpec()

	var documented map[string]bool
	for _, field := range spec.Document.Fields {
		if field.Name != "objective" {
			continue
		}
		documented = make(map[string]bool, len(field.Fields))
		for _, f := range field.Fields {
			documented[f.Name] = true
		}
	}
	if documented == nil {
		t.Fatal(`GenerateFullSpec().Document.Fields has no "objective" entry`)
	}

	actual := jsonFieldNames(reflect.TypeOf(game.ObjectiveDoc{}))
	for name := range actual {
		if !documented[name] {
			t.Errorf("ObjectiveDoc has field %q, which the objective spec does not document", name)
		}
	}
	for name := range documented {
		if !actual[name] {
			t.Errorf("the objective spec documents %q, which ObjectiveDoc no longer has", name)
		}
	}
}
