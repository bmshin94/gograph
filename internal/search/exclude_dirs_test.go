package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ozgurcd/gograph/internal/buildctx"
	"github.com/ozgurcd/gograph/internal/graph"
)

func TestDeclarationBaselinePreservesExcludeDirs(t *testing.T) {
	t.Setenv("GOWORK", "off")
	root := t.TempDir()
	for file, content := range map[string]string{"go.mod": "module example.com/selection\ngo 1.27.0\n", "good.go": "package good\nfunc Keep() {}\n", "broken/bad.go": "not Go source"} {
		path := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	config, err := buildctx.Resolve(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	selection := graph.CaptureBuildSelection(config.BuildContext())
	selection.ExcludeDirs = []string{"broken"}
	baseline, err := buildDeclarationBaseline(context.Background(), root, selection)
	if err != nil {
		t.Fatal(err)
	}
	if len(baseline.Files) != 1 || len(baseline.Build.Selection.ExcludeDirs) != 1 {
		t.Fatalf("lost selection: %+v", baseline)
	}
	selection.ExcludeDirs = []string{"../outside"}
	if _, err := buildDeclarationBaseline(context.Background(), root, selection); err == nil {
		t.Fatal("accepted unsafe persisted exclusions")
	}
}
