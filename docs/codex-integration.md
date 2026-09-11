# Gograph MCP with Codex

Install Gograph and check `gograph version`. For the excluded-directory symlink
fix, use v1.7.2 or newer. Install Go as well: precise analysis needs its toolchain.

Build the project and register its local stdio server:

```bash
cd /absolute/path/to/project
gograph build . --precise --strict --exclude-dirs=.claude/skills
codex mcp add gograph-project -- gograph mcp /absolute/path/to/project --exclude-dirs=.claude/skills
codex mcp list
```

Omit the exclusion from both commands if it is unnecessary. Use a distinct
server name and absolute project path for each repository. If Codex cannot find
`gograph`, replace the command with its absolute executable path (for example,
`/opt/homebrew/bin/gograph` on an Apple Silicon Homebrew installation).

Alternatively, merge this table into your existing `~/.codex/config.toml`;
do not create a duplicate table or overwrite unrelated settings:

```toml
[mcp_servers.gograph-project]
command = "gograph"
args = ["mcp", "/absolute/path/to/project", "--exclude-dirs=.claude/skills"]
startup_timeout_sec = 120
tool_timeout_sec = 120
```

Codex supports stdio registrations through `codex mcp add` and TOML server
configuration, including optional timeouts. See the
[official MCP documentation](https://learn.chatgpt.com/docs/extend/mcp?surface=cli).

Restart Codex's MCP connection (or restart Codex) after installation or argument
changes. Ask it to call `gograph_capabilities` and check the server `version`
and `analysis_build_context.exclude_dirs`, then call `gograph_stale` and
`gograph_query` on a known symbol. `codex mcp list` verifies registration, not
successful graph analysis. Reuse the build's tags and Go environment as well.

Directory exclusions do not grant permission to follow linked Go inputs. See
[the symlink boundary and limitations](build-selection.md#symlinked-skills-and-tooling-directories).
MCP refresh is in-memory by default; use `--persist-refresh` only if you want it
to update the CLI's persisted graph. No hosted Gograph endpoint or API key is
needed for this local server.
