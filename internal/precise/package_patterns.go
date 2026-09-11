package precise

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ozgurcd/gograph/internal/buildctx"
	"github.com/ozgurcd/gograph/internal/graph"
)

// Explicit package patterns keep unrelated broken packages out of packages.Load.
// Dependencies are still loaded normally: excluding an imported package cannot
// make its type errors safe to ignore.
func packagePatterns(g *graph.Graph, config buildctx.Config) ([]string, error) {
	if len(config.ExcludeDirs()) == 0 {
		return []string{"./..."}, nil
	}
	seen := make(map[string]bool)
	for _, file := range g.Files {
		rel := filepath.ToSlash(filepath.Clean(file.Path))
		if filepath.IsAbs(file.Path) || rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, fmt.Errorf("invalid selected source path %q", file.Path)
		}
		if config.Excludes(rel) {
			return nil, fmt.Errorf("graph contains excluded source %q; rebuild with the same selection", file.Path)
		}
		dir := filepath.ToSlash(filepath.Dir(file.Path))
		pattern := "./" + dir
		if dir == "." {
			pattern = "."
		}
		seen[pattern] = true
	}
	patterns := make([]string, 0, len(seen))
	for pattern := range seen {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no packages remain after directory exclusions")
	}
	return patterns, nil
}
