package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

func TestMCPReadStructuredAnswerAndRefusal(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte("package fixture\nfunc Answer() int { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &graph.Graph{Root: root, Symbols: []graph.SymbolNode{{ID: "example.com/fixture::Answer", Name: "Answer", Kind: graph.KindFunction, File: "fixture.go", Line: 2, EndLine: 2}}}
	handlers := setupHandlers(t, g)
	for _, tool := range []string{"gograph_source", "gograph_context"} {
		for _, symbol := range []string{"Answer", "example.com/fixture::Answer", "Absent"} {
			t.Run(tool+"/"+symbol, func(t *testing.T) {
				req := mcp.CallToolRequest{}
				req.Params.Arguments = map[string]any{"symbol": symbol}
				result, err := handlers[tool](context.Background(), req)
				if err != nil || result == nil {
					t.Fatalf("read: %v, %v", result, err)
				}
				data, err := json.Marshal(result.StructuredContent)
				if err != nil {
					t.Fatal(err)
				}
				var body map[string]any
				if err := json.Unmarshal(data, &body); err != nil {
					t.Fatal(err)
				}
				if symbol != "Absent" {
					if result.IsError || !strings.Contains(string(data), "func Answer() int { return 42 }") {
						t.Fatalf("structured read lost source: %s", data)
					}
					expected, err := search.Source(g, root, symbol)
					if err != nil || body["source"] != expected || body["schema_version"] != "gograph.read.v1" || body["status"] != "ok" {
						t.Fatalf("structured source differs from shared CLI source: %s, %v", data, err)
					}
					if tool == "gograph_context" && body["node"] == nil {
						t.Fatalf("structured context lost node: %s", data)
					}
				} else if !result.IsError || !strings.Contains(string(data), "Absent") || !strings.Contains(string(data), "not found") {
					t.Fatalf("structured read lost refusal: isError=%v %s", result.IsError, data)
				}
				if body["graph_state"] == nil {
					t.Fatalf("read lost provenance: %s", data)
				}
			})
		}
	}
}

func TestMCPReadReportsIndexedButUnreadableSource(t *testing.T) {
	root := t.TempDir()
	g := &graph.Graph{Root: root, Build: &graph.BuildMetadata{Complete: true, Precision: graph.PrecisionAST}, Symbols: []graph.SymbolNode{{ID: "example.com/fixture::MissingFile", Name: "MissingFile", Kind: graph.KindFunction, File: "missing.go", Line: 1, EndLine: 1}}}
	handlers := setupHandlers(t, g)
	for _, tool := range []string{"gograph_source", "gograph_context"} {
		t.Run(tool, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"symbol": "MissingFile"}
			result, err := handlers[tool](context.Background(), req)
			if err != nil || result == nil {
				t.Fatalf("read: %v %v", result, err)
			}
			data, err := json.Marshal(result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				Reason     string `json:"reason"`
				GraphState struct {
					Diagnostic string `json:"read_diagnostic"`
					Precision  string `json:"precision"`
				} `json:"graph_state"`
			}
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(body.Reason, "missing.go") || body.GraphState.Diagnostic != body.Reason || body.GraphState.Precision != "ast" {
				t.Fatalf("indexed unreadable source must be disclosed on both axes: %s", data)
			}
		})
	}
}
