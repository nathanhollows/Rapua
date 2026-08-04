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
		"location.some-slug.visited",
		"location.some-slug.checked_in",
		"group.Some Group.completed",
	}
	for _, name := range prefix {
		if !isBuiltInVar(name) {
			t.Errorf("isBuiltInVar(%q) = false, want true", name)
		}
	}

	// Explicitly NOT built-in.
	notBuiltin := []string{
		"game.status",
		"location.some-slug.other",
		"group.Some Group.other",
		"",
		"something.else",
		"points.something",
		"player.badges", // Stage 2 — not implemented yet
		"run.finished_at",
	}
	for _, name := range notBuiltin {
		if isBuiltInVar(name) {
			t.Errorf("isBuiltInVar(%q) = true, want false", name)
		}
	}
}
