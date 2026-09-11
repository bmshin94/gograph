package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozgurcd/gograph/internal/scanner"
)

func TestExcludedDirectoryLinksConfinesPermissionAndRechecks(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/links\n\ngo 1.27.0\n")
	mustWrite(t, filepath.Join(root, "main.go"), "package main\n")
	t.Setenv("GOWORK", "off")
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "SKILL.md"), "# Skill\n")
	parent := filepath.Join(root, ".claude", "skills")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "example")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	for _, dirs := range [][]string{nil, {".claude/skills-other"}, {"skills"}} {
		if _, err := scanner.ExcludedDirectoryLinks(root, dirs); !scanner.IsUnsafeSourceFileError(err) {
			t.Fatalf("unselected link accepted with %v: %v", dirs, err)
		}
	}
	links, err := scanner.ExcludedDirectoryLinks(root, []string{".claude/skills"})
	if err != nil || len(links) != 1 {
		t.Fatalf("excluded directory link = %v, %v", links, err)
	}
	if err := scanner.ValidateMaskedSourceLinks(root, links); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(parent, "added")); err != nil {
		t.Fatal(err)
	}
	if err := scanner.ValidateMaskedSourceLinks(root, links); !scanner.IsUnsafeSourceFileError(err) {
		t.Fatalf("new unmasked link escaped recheck: %v", err)
	}
}

func TestExcludedDirectoryLinksStillRejectsBuildInputs(t *testing.T) {
	for _, name := range []string{"linked.go", "go.mod", "go.sum", "go.work", "native.c"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("GOWORK", "off")
			root := t.TempDir()
			mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/links\n\ngo 1.27.0\n")
			parent := filepath.Join(root, ".claude", "skills")
			if err := os.MkdirAll(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), filepath.Join(parent, name)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if _, err := scanner.ExcludedDirectoryLinks(root, []string{".claude"}); !scanner.IsUnsafeSourceFileError(err) {
				t.Fatalf("linked build input %s accepted: %v", name, err)
			}
		})
	}
}
