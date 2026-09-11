# Directory exclusions

Use directory exclusions when unrelated packages prevent precise analysis:

```bash
gograph build . --precise --strict --exclude-dirs=legacy,examples/broken
gograph mcp . --exclude-dirs=legacy,examples/broken
gograph stale --exclude-dirs=legacy,examples/broken --json
```

`--exclude-dirs dir1,dir2` and repeated flags are also accepted. Values are
literal paths relative to the analyzed repository, not the shell's working
directory. `legacy` excludes `legacy/` and its descendants, but not
`legacy-other/` or `service/legacy/`. Leading `./`, trailing `/`, ordering,
duplicates, and redundant children are normalized. Absolute paths, `..`
components, globs, empty entries, and excluding the repository root are rejected.
Use forward slashes in paths, including on Windows. Nonexistent paths are allowed
so a selection can survive branch changes.

Exclusions apply before AST parsing and before selecting precise production and
test packages. Precise analysis loads explicit remaining package directories
instead of `./...`. The remaining code must still compile. If an included package
imports an excluded package, Go still loads that dependency; exclusion cannot
hide its type errors. Exclusions do not disable source-path, symlink, module,
workspace, or other Go-input safety checks. Invalid required `go.mod` / `go.work`
metadata must still be corrected.

The normalized list is stored in `build.selection.exclude_dirs` and bound into
both build-context and source fingerprints. Completeness describes the selected
code, not the excluded trees. Rebuild without the flag to restore the full
selection. Excluded source edits alone do not invalidate the selected graph;
as with other unindexed dependencies, rebuild after modifying an excluded package
that included code imports. Required module/build metadata remains fingerprinted.

CLI queries and ordinary `stale` checks describe the persisted selection.
`doctor --json` also reports and checks that selection in `repository.exclude_dirs`.
`stale --exclude-dirs=...` checks a different explicit selection when needed.
If using build tags, continue supplying the matching `--tags` value as well.

For MCP, supply the same exclusions at server startup. Automatic startup builds,
in-memory refreshes, optional `--persist-refresh` publication, and baselines keep
that startup selection. `gograph_capabilities.analysis_build_context.exclude_dirs`
reports it. Exclusions are a server-level setting, not per-query tool arguments.
Restart an existing MCP server after changing its startup configuration.

## Workspace members

Declare member-relative exclusions in `.gograph-workspace.yml`:

```yaml
schema_version: gograph.workspace-manifest.v1
name: example
repositories:
  - id: api
    path: services/api
    precision: precise
    exclude_dirs: [legacy, examples/broken]
```

Build the member with matching `--exclude-dirs`, or use
`gograph workspace build --refresh-members` to apply the manifest selection.
Workspace status and queries use the same selection through CLI and MCP.
Changing it invalidates member/overlay freshness. The normal workspace build
still does not mutate members without `--refresh-members`.

Machine validation does not silently adopt exclusions from an artifact as a
full-repository selection. A validation request that expects the full repository
rejects a graph built with exclusions as a selection mismatch.
