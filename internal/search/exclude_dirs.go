package search

import (
	"context"

	"github.com/ozgurcd/gograph/internal/buildctx"
	"github.com/ozgurcd/gograph/internal/graph"
)

// Persisted queries describe the recorded selection. Explicit build and MCP
// startup selections still compare against their requested configuration.
func staleWithRecordedExclusions(g *graph.Graph, root string) StaleResult {
	config, _ := buildctx.ResolveOrDefault(context.Background(), root)
	config, err := config.WithExcludeDirs(g.Build.Selection.ExcludeDirs)
	if err != nil {
		return StaleResult{IsStale: true, BuildContextChanged: true}
	}
	return StaleWithConfig(g, root, config)
}
