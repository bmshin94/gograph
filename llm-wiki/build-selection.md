---
title: Build selection and excluded directory symlinks
type: decision
status: current
updated: 2026-09-11
sources: []
---

# Build selection and excluded directory symlinks

Repository exclusions are literal root-relative subtrees, recorded in the graph's build selection and input fingerprints. CLI and MCP use the same selection mechanisms; MCP startup must explicitly supply the exclusions and retains them across refresh. Workspace members declare their own exclusions. Completeness covers selected code, not the omitted trees. See [the user-facing contract](../docs/build-selection.md).

## Security boundary

The v1.7.1 implementation excluded packages but still rejected every descendant directory symlink during precise preflight. The reported `.claude/skills/example -> ../../.agents/skills/example` case therefore remained broken. Commit `6eeb07b` (prepared release tag v1.7.2 at `dc63800`) corrects that case without treating an exclusion as permission to follow links.

Only directory symlinks beneath explicitly excluded real directories can be masked. The preflight collects their canonical link paths without enumerating target contents. A private temporary Go deletion overlay makes those paths absent during both production and test loading, including direct, transitive, and local-replacement lookup. The file is removed after enrichment. Its random execution filename is not persisted build identity. An existing Go overlay from process flags or persisted GOENV is rejected rather than silently overridden.

Recognized linked Go/C/assembly inputs and module/workspace metadata remain errors even inside exclusions. Unexcluded directory links remain errors. Post-load rechecks authorize only the exact previously masked paths, so newly introduced links are not silently accepted. Real excluded packages remain importable and must compile; masked links cannot supply dependencies. Source and artifact confinement remain unchanged. The standalone CLI/MCP `doc` operation retains its independent strict preflight and does not apply build exclusions.

## Evidence and limits

Scanner tests cover selection boundaries, protected input names, and new-link rechecks. Actual CLI tests cover the reported relative skill layout, external linked targets, `.claude` and `.claude/skills` exclusions, and direct/test/transitive/replacement import refusal without leaking target diagnostic markers. Actual stdio MCP tests assert matching CLI query rows, retained precise state after refresh, and unchanged persisted artifact bytes. Overlay tests cover deletion serialization, cleanup, and process/persisted operator-overlay conflicts. These are bounded regression results, not a claim of a complete OS sandbox or closed-world dependency resolution.
