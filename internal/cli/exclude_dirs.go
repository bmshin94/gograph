package cli

import (
	"fmt"
	"strings"

	"github.com/ozgurcd/gograph/internal/buildctx"
)

func parseExcludeDirsOption(args []string, index int, dirs *[]string) (bool, int, error) {
	arg := args[index]
	if arg != "--exclude-dirs" && !strings.HasPrefix(arg, "--exclude-dirs=") {
		return false, index, nil
	}
	value := strings.TrimPrefix(arg, "--exclude-dirs=")
	if arg == "--exclude-dirs" {
		index++
		if index >= len(args) || strings.HasPrefix(args[index], "--") {
			return true, index, fmt.Errorf("--exclude-dirs requires comma-separated repository-relative directories")
		}
		value = args[index]
	}
	values := append(append([]string(nil), (*dirs)...), strings.Split(value, ",")...)
	normalized, err := buildctx.NormalizeExcludeDirs(values)
	if err != nil {
		return true, index, fmt.Errorf("invalid --exclude-dirs: %w", err)
	}
	*dirs = normalized
	return true, index, nil
}

func buildSelectionOptions(tags []string, exclusions ...[]string) buildctx.ResolveOptions {
	options := buildctx.ResolveOptions{BuildTags: tags}
	for _, dirs := range exclusions {
		options.ExcludeDirs = append(options.ExcludeDirs, dirs...)
	}
	return options
}

func printExcludeDirsHelp(command string) {
	if command != "" && command != "build" && command != "mcp" && command != "stale" {
		return
	}
	fmt.Println(`
DIRECTORY EXCLUSIONS
  --exclude-dirs=dir1,dir2  Exclude literal repository-relative directory subtrees.
  Also accepts --exclude-dirs dir1,dir2 and repeated flags. No globs, absolute
  paths, parent traversal, or repository-root exclusion. Unlisted directories
  remain selected; names match from the analysis root, not at every depth.
  Applies to AST and precise targets, including tests. Imported dependencies
  still must type-check; source safety and Go metadata checks remain enforced.
  Directory symlinks beneath excluded real directories are masked from precise
  Go loading (e.g. --exclude-dirs=.claude/skills); targets are not followed.
  Linked Go inputs/metadata remain errors. An existing Go -overlay conflicts
  with this masking. The standalone doc operation keeps its strict preflight.
  Build selection/fingerprints record exclusions. Pass the same flag to mcp
  to retain them across refreshes. Stale uses recorded exclusions by default.
  Rebuild without the flag to restore full selection. Workspace members use
  exclude_dirs: [dir1, dir2] in .gograph-workspace.yml.`)
}
