package search_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
)

func TestSourceRefusesAmbiguityAndOversize(t *testing.T) {
	root := t.TempDir()
	g := &graph.Graph{Symbols: []graph.SymbolNode{
		{ID: "example.com/a::Answer", Name: "Answer", File: "a.go", Line: 2, EndLine: 2},
		{ID: "example.com/b::Answer", Name: "Answer", File: "b.go", Line: 2, EndLine: 2},
	}}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\nfunc Answer() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := search.Source(g, root, "Answer")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "example.com/a::Answer") || !strings.Contains(err.Error(), "example.com/b::Answer") {
		t.Fatalf("ambiguity must name candidates: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n//"+strings.Repeat("x", 65537)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = search.Source(g, root, "example.com/a::Answer")
	if err == nil || !strings.Contains(err.Error(), "bytes") || !strings.Contains(err.Error(), "65536") {
		t.Fatalf("oversize must name size and budget: %v", err)
	}
}

func TestSourceReturnsErrorWhenEveryMatchIsUnreadable(t *testing.T) {
	root := t.TempDir()
	g := &graph.Graph{Symbols: []graph.SymbolNode{{
		ID:      "example.com/project::Missing",
		Name:    "Missing",
		Kind:    graph.KindFunction,
		File:    "missing.go",
		Line:    1,
		EndLine: 2,
	}}}

	source, err := search.Source(g, root, "Missing")
	if err == nil {
		t.Fatalf("expected an unreadable source error, got source %q", source)
	}
	if !strings.Contains(err.Error(), filepath.Join("missing.go")) {
		t.Fatalf("error should identify the unreadable file: %v", err)
	}
}
