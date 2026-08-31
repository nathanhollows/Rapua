package game //nolint:testpackage // white-box test for internal isBuiltInVar

import "testing"

// TestIsBuiltInVar_CanonicalSet ensures the linter's built-in variable
// detection matches the canonical set. Adding or removing a built-in in
// isBuiltInVar without updating the spec and resolver will cause this
// test to fail. objective.<slug> is the only built-in namespace: conditions
// are truthy-only, so the numeric built-ins went with the comparison operators.
func TestIsBuiltInVar_CanonicalSet(t *testing.T) {
	// The whole built-in namespace is the objective prefix.
	prefix := []string{
		"objective.some-slug",
	}
	for _, name := range prefix {
		if !isBuiltInVar(name) {
			t.Errorf("isBuiltInVar(%q) = false, want true", name)
		}
	}

	// Explicitly NOT built-in.
	notBuiltin := []string{
		"player.points",
		"points",
		"run.started_at",
		"game.team_count",
		"game.status",
		"location.some-slug.visited",
		"location.some-slug.checked_in",
		"group.Some Group.completed",
		"",
		"something.else",
		"points.something",
		"run.finished_at",
		"objective.",
	}
	for _, name := range notBuiltin {
		if isBuiltInVar(name) {
			t.Errorf("isBuiltInVar(%q) = true, want false", name)
		}
	}
}

func TestIsReservedVarName(t *testing.T) {
	reserved := []string{
		"objective.find-maisie",
		"objective.any-slug",
	}
	for _, name := range reserved {
		if !IsReservedVarName(name) {
			t.Errorf("IsReservedVarName(%q) = false, want true", name)
		}
	}

	notReserved := []string{
		"player.points",
		"run.started_at",
		"my_var",
		"objective.", // empty slug
		"",
	}
	for _, name := range notReserved {
		if IsReservedVarName(name) {
			t.Errorf("IsReservedVarName(%q) = true, want false", name)
		}
	}
}
