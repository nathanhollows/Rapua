package game //nolint:testpackage // white-box test for internal isBuiltInVar

import "testing"

// TestIsBuiltInVar_CanonicalSet ensures the linter's built-in variable
// detection matches the canonical set. Adding or removing a built-in in
// isBuiltInVar without updating the spec and resolver will cause this
// test to fail.
func TestIsBuiltInVar_CanonicalSet(t *testing.T) {
	// Exact-match builtins (no wildcards). "points" is the pre-respine
	// spelling of player.points and is still accepted.
	exact := []string{
		"player.points",
		"points",
		"run.started_at",
		"game.team_count",
	}
	for _, name := range exact {
		if !isBuiltInVar(name) {
			t.Errorf("isBuiltInVar(%q) = false, want true", name)
		}
	}

	// Prefix/wildcard builtins.
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
