package specgen_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/game"
	"github.com/nathanhollows/Rapua/v7/internal/specgen"
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

// TestGenerateFullSpec_Version checks that the generated spec reports version "v7".
func TestGenerateFullSpec_Version(t *testing.T) {
	spec := specgen.GenerateFullSpec()
	if spec.Version != "v7" {
		t.Errorf("GenerateFullSpec().Version = %q, want %q", spec.Version, "v7")
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
		"location_content", "navigation", "start", "finish",
	}
	for _, ctx := range expected {
		if !contextValues[ctx] {
			t.Errorf("GenerateFullSpec().Contexts missing %q", ctx)
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

// TestGenerateBlockSpecs_WhenOnAll verifies every block spec references "when" in shared_fields.
func TestGenerateBlockSpecs_WhenOnAll(t *testing.T) {
	for _, spec := range specgen.GenerateBlockSpecs() {
		found := false
		for _, name := range spec.SharedFields {
			if name == "when" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("block %q: missing \"when\" in shared_fields", spec.Type)
		}
	}
}

// TestGenerateBlockSpecs_SetsOnInteractiveOnly verifies "sets" appears in shared_fields
// on interactive block types and not on content-only block types.
func TestGenerateBlockSpecs_SetsOnInteractiveOnly(t *testing.T) {
	interactiveTypes := map[string]bool{
		"quiz": true, "password": true, "pincode": true, "broker": true,
		"sorting": true, "photo": true, "checklist": true, "rating": true, "free_text": true,
	}

	for _, spec := range specgen.GenerateBlockSpecs() {
		hasSets := false
		for _, name := range spec.SharedFields {
			if name == "sets" {
				hasSets = true
				break
			}
		}
		if interactiveTypes[spec.Type] && !hasSets {
			t.Errorf("interactive block %q: missing \"sets\" in shared_fields", spec.Type)
		}
		if !interactiveTypes[spec.Type] && hasSets {
			t.Errorf("content block %q: should not have \"sets\" in shared_fields", spec.Type)
		}
	}
}

// TestGenerateFullSpec_BlockSharedFields verifies block_shared_fields contains "when" and "sets".
func TestGenerateFullSpec_BlockSharedFields(t *testing.T) {
	spec := specgen.GenerateFullSpec()
	names := make(map[string]bool, len(spec.BlockSharedFields))
	for _, f := range spec.BlockSharedFields {
		names[f.Name] = true
	}
	for _, want := range []string{"when", "sets"} {
		if !names[want] {
			t.Errorf("block_shared_fields missing %q", want)
		}
	}
}

// TestGenerateFullSpec_WhenOnLocationAndGroup verifies the location and structure (group)
// schemas both contain a "when" field.
func TestGenerateFullSpec_WhenOnLocationAndGroup(t *testing.T) {
	spec := specgen.GenerateFullSpec()

	// Find location schema
	var locationFields []game.FieldSpec
	var structureFields []game.FieldSpec
	for _, f := range spec.Document.Fields {
		switch f.Name {
		case "location":
			locationFields = f.Fields
		case "structure":
			structureFields = f.Fields
		}
	}

	hasWhen := func(fields []game.FieldSpec) bool {
		for _, f := range fields {
			if f.Name == "when" {
				return true
			}
		}
		return false
	}

	if !hasWhen(locationFields) {
		t.Error("location schema missing \"when\" field")
	}
	if !hasWhen(structureFields) {
		t.Error("structure (group) schema missing \"when\" field")
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

	expected := []string{"points", "game.team_count"}
	for _, name := range expected {
		if !varNames[name] {
			t.Errorf("built_in_vars missing %q", name)
		}
	}
}

// TestGenerateFullSpec_ConditionOpsEnum verifies condition_ops enum is non-empty and
// contains all expected operators.
func TestGenerateFullSpec_ConditionOpsEnum(t *testing.T) {
	spec := specgen.GenerateFullSpec()
	if len(spec.Enums.ConditionOps) == 0 {
		t.Fatal("GenerateFullSpec().Enums.ConditionOps is empty")
	}

	ops := make(map[string]bool, len(spec.Enums.ConditionOps))
	for _, op := range spec.Enums.ConditionOps {
		ops[op.Value] = true
	}

	expected := []string{"eq", "neq", "gt", "lt", "gte", "lte", "in", "not_in"}
	for _, op := range expected {
		if !ops[op] {
			t.Errorf("condition_ops missing operator %q", op)
		}
	}
}
