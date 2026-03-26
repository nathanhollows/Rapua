---
title: "Versioning"
sidebar: true
order: 2
---

# Versioning

Rapua follows [Semantic Versioning](https://semver.org/) strictly. The version is the source of truth for compatibility — even though Rapua is a web app with no external library consumers, the Go module path encodes the major version and all internal imports must agree.

## Version sources

The version is defined in three places that must stay in sync:

| Source | Location | Example |
|--------|----------|---------|
| Code constant | `cmd/rapua/main.go` → `version` | `"v7.0.0"` |
| Changelog | `docs/changelog.md` → first `## X.Y.Z` heading | `## 7.0.0` |
| Go module path | `go.mod` → `module` line | `github.com/nathanhollows/Rapua/v7` |

A test in `cmd/rapua/main_test.go` enforces that all three agree:
- `TestVersionMatchesChangelog` checks the code constant against the changelog.
- `TestModuleVersionMatchesChangelog` checks the changelog major version against the `/vN` suffix in `go.mod`.

## When to bump

- **PATCH** (`7.0.0` → `7.0.1`): Bug fixes, typo corrections, dependency updates. No new features, no breaking changes.
- **MINOR** (`7.0.1` → `7.1.0`): New features, new endpoints, new block types — anything additive that doesn't break existing behaviour.
- **MAJOR** (`7.1.0` → `8.0.0`): Breaking changes to exported Go types, DB schema changes that alter stored data shape (e.g., enum type changes), renamed or removed public interfaces.

## How to bump a major version

A major version bump requires updating every internal import path. This is mechanical but touches many files.

### Steps

1. **Update the changelog** — add a `## X.0.0` heading at the top of `docs/changelog.md`.

2. **Update the code version** — set the `version` constant in `cmd/rapua/main.go`.

3. **Update the module path and all imports** — two commands from the project root:
   ```bash
   # Update go.mod module path
   sed -i 's|Rapua/vOLD|Rapua/vNEW|g' go.mod

   # Update every Go and templ import
   find . \( -name '*.go' -o -name '*.templ' \) -exec sed -i 's|Rapua/vOLD|Rapua/vNEW|g' {} +
   ```

4. **Verify** — build and run the full test suite:
   ```bash
   go build ./... && go test ./...
   ```
   `TestVersionMatchesChangelog` and `TestModuleVersionMatchesChangelog` will catch any mismatch between the three version sources.

### Minor / patch bumps

For minor and patch bumps, only steps 1 and 2 are needed — the module path doesn't change within a major version.
