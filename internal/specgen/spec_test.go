package specgen_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/specgen"
	"github.com/stretchr/testify/assert"
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

// TestGenerateFullSpec_HasEnums checks routing and completion enums are non-empty.
func TestGenerateFullSpec_HasEnums(t *testing.T) {
	spec := specgen.GenerateFullSpec()
	if len(spec.Enums.Routing) == 0 {
		t.Error("GenerateFullSpec().Enums.Routing is empty")
	}
	if len(spec.Enums.Completion) == 0 {
		t.Error("GenerateFullSpec().Enums.Completion is empty")
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

// TestSpecStaleness_IgnoresBaseBlockFields verifies that the staleness check
// does not flag the anonymous BaseBlock fields as missing from specs.
func TestSpecStaleness_IgnoresBaseBlockFields(t *testing.T) {
	registered := blocks.GetRegisteredBlocks()
	for _, reg := range registered {
		sp, ok := reg.Prototype.(game.SpecProvider)
		if !ok {
			continue
		}
		spec := sp.GetSpec()
		specFields := make(map[string]bool, len(spec.Fields))
		for _, f := range spec.Fields {
			specFields[f.Name] = true
		}
		// BaseBlock fields "id" and "type" are promoted — they must NOT be in spec.Fields.
		if specFields["id"] {
			t.Errorf("block %q: GetSpec().Fields should not include promoted field \"id\"", reg.BlockType)
		}
		if specFields["type"] {
			t.Errorf("block %q: GetSpec().Fields should not include promoted field \"type\"", reg.BlockType)
		}
	}
}

// TestGenerateBlockSpecs_SetsMatchesTheBlock checks the published spec against the
// block itself. A list kept here would be another copy to forget to update, which
// is how choice and scan came to advertise no "sets" while the runtime honoured it.
func TestGenerateBlockSpecs_SetsMatchesTheBlock(t *testing.T) {
	supportsSets := make(map[string]bool)
	for _, reg := range blocks.GetRegisteredBlocks() {
		supportsSets[reg.BlockType] = reg.Prototype.SupportsVariableSets()
	}

	for _, spec := range specgen.GenerateBlockSpecs() {
		hasSets := false
		for _, name := range spec.SharedFields {
			if name == "sets" {
				hasSets = true
				break
			}
		}
		if want := supportsSets[spec.Type]; hasSets != want {
			t.Errorf("block %q: spec advertises sets=%v, block reports %v", spec.Type, hasSets, want)
		}
	}
}

// TestGenerateFullSpec_BlockSharedFields verifies block_shared_fields contains "sets" and "points".
func TestGenerateFullSpec_BlockSharedFields(t *testing.T) {
	spec := specgen.GenerateFullSpec()
	names := make(map[string]bool, len(spec.BlockSharedFields))
	for _, f := range spec.BlockSharedFields {
		names[f.Name] = true
	}
	for _, want := range []string{"sets", "points"} {
		if !names[want] {
			t.Errorf("block_shared_fields missing %q", want)
		}
	}
}

// TestGenerateBlockSpecs_PointsOnlyOnInteractiveBlocks proves "points" is
// documented exactly on blocks with a completion event whose Points field is
// actually honoured (RequiresValidation && SupportsPoints), matching what the
// runtime actually awards it for: content blocks (markdown, alert, divider,
// etc.) never complete, and Broker completes but always resets Points to 0
// (it uses per-tier costs instead): neither should advertise the field.
func TestGenerateBlockSpecs_PointsOnlyOnInteractiveBlocks(t *testing.T) {
	registered := blocks.GetRegisteredBlocks()
	wantPoints := make(map[string]bool, len(registered))
	for _, reg := range registered {
		wantPoints[reg.BlockType] = reg.Prototype.RequiresValidation() && reg.Prototype.SupportsPoints()
	}

	for _, spec := range specgen.GenerateBlockSpecs() {
		hasPoints := false
		for _, f := range spec.SharedFields {
			if f == "points" {
				hasPoints = true
			}
		}
		if want := wantPoints[spec.Type]; hasPoints != want {
			t.Errorf("block %q: SharedFields has points=%v, want %v (RequiresValidation && SupportsPoints)",
				spec.Type, hasPoints, want)
		}
	}
}

// TestBrokerBlock_DoesNotSupportPoints locks in the specific exception the
// above test relies on: Broker requires validation but must not advertise
// the shared points field, since UpdateBlockData always resets it to 0.
func TestBrokerBlock_DoesNotSupportPoints(t *testing.T) {
	b := &blocks.BrokerBlock{}
	assert.True(t, b.RequiresValidation())
	assert.False(t, b.SupportsPoints())
}

// TestGenerateFullSpec_DependsOnObjective verifies the objective schema carries
// the depends field. Groups have no condition of their own: a group's gate is
// the reachability of the objectives inside it.
func TestGenerateFullSpec_DependsOnObjective(t *testing.T) {
	spec := specgen.GenerateFullSpec()

	var objectiveFields []game.FieldSpec
	var structureFields []game.FieldSpec
	for _, f := range spec.Document.Fields {
		switch f.Name {
		case "objective":
			objectiveFields = f.Fields
		case "structure":
			structureFields = f.Fields
		}
	}

	hasField := func(fields []game.FieldSpec, name string) bool {
		for _, f := range fields {
			if f.Name == name {
				return true
			}
		}
		return false
	}

	if !hasField(objectiveFields, "depends") {
		t.Error("objective schema missing \"depends\" field")
	}
	if hasField(objectiveFields, "when") || hasField(structureFields, "when") {
		t.Error("schemas still publish a \"when\" field")
	}
}

// TestGenerateFullSpec_BuiltInVars verifies built_in_vars is non-empty and contains
// expected variable names.
func TestGenerateFullSpec_BuiltInVars(t *testing.T) {
	spec := specgen.GenerateFullSpec()
	if len(spec.BuiltInVars) == 0 {
		t.Fatal("GenerateFullSpec().BuiltInVars is empty")
	}

	varNames := make(map[string]bool, len(spec.BuiltInVars))
	for _, v := range spec.BuiltInVars {
		varNames[v.Var] = true
		if v.Type == "" {
			t.Errorf("built_in_var %q has empty Type", v.Var)
		}
		if v.Description == "" {
			t.Errorf("built_in_var %q has empty Description", v.Var)
		}
	}

	expected := []string{"objective.<slug>"}
	for _, name := range expected {
		if !varNames[name] {
			t.Errorf("built_in_vars missing %q", name)
		}
	}
}
