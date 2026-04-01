package blocks_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v7/blocks"
)

// TestFieldSpecDrift verifies that every json-tagged field on a registered block's
// struct has a corresponding FieldDef entry in that block's FieldSpec().
//
// This catches the case where a developer adds a field to a block struct
// (giving it a json tag) but forgets to update FieldSpec().
func TestFieldSpecDrift(t *testing.T) {
	t.Parallel()

	for _, blockType := range blocks.GetAllRegisteredTypes() {
		blockType := blockType // capture for parallel sub-tests
		t.Run(blockType, func(t *testing.T) {
			t.Parallel()

			registration := blocks.GetRegisteredBlock(blockType)
			if registration == nil {
				t.Fatalf("GetRegisteredBlock(%q) returned nil", blockType)
			}

			provider, ok := registration.Instance.(blocks.FieldSpecProvider)
			if !ok {
				t.Errorf("block type %q does not implement FieldSpecProvider", blockType)
				return
			}

			spec := provider.FieldSpec()
			specNames := flatFieldNames(spec)

			// Reflect on the underlying struct to collect json-tagged field names.
			structType := reflect.TypeOf(registration.Instance)
			if structType.Kind() == reflect.Ptr {
				structType = structType.Elem()
			}

			jsonNames := collectJSONFieldNames(structType)

			for _, name := range jsonNames {
				if !specNames[name] {
					t.Errorf(
						"block %q: struct field json:%q has no matching FieldDef in FieldSpec()",
						blockType, name,
					)
				}
			}
		})
	}
}

// TestGenerateSpec verifies that GenerateSpec() returns a non-empty string
// containing every registered block type name.
func TestGenerateSpec(t *testing.T) {
	t.Parallel()

	spec := blocks.GenerateSpec()
	if spec == "" {
		t.Fatal("GenerateSpec() returned empty string")
	}

	for _, blockType := range blocks.GetAllRegisteredTypes() {
		if !strings.Contains(spec, blockType) {
			t.Errorf("GenerateSpec() output does not mention block type %q", blockType)
		}
	}
}

// TestGetFieldSpec_UnknownType verifies that GetFieldSpec returns nil for unknown types.
func TestGetFieldSpec_UnknownType(t *testing.T) {
	t.Parallel()
	if got := blocks.GetFieldSpec("does_not_exist"); got != nil {
		t.Errorf("GetFieldSpec(unknown) = %v, want nil", got)
	}
}

// flatFieldNames returns the set of all top-level field names in a FieldSpec slice.
func flatFieldNames(fields []blocks.FieldDef) map[string]bool {
	names := make(map[string]bool, len(fields))
	for _, f := range fields {
		names[f.Name] = true
	}
	return names
}

// collectJSONFieldNames returns all json key names from a struct type,
// recursing into embedded (anonymous) fields and skipping json:"-" tags.
func collectJSONFieldNames(t reflect.Type) []string {
	var names []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")

		if field.Anonymous {
			// Embedded struct — recurse.
			embedded := field.Type
			if embedded.Kind() == reflect.Ptr {
				embedded = embedded.Elem()
			}
			if embedded.Kind() == reflect.Struct {
				names = append(names, collectJSONFieldNames(embedded)...)
			}
			continue
		}

		if tag == "" || tag == "-" {
			continue
		}

		name := strings.SplitN(tag, ",", 2)[0]
		if name == "-" || name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}
