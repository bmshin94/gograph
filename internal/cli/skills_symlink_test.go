package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ozgurcd/gograph/internal/graph"
)

func skillsTestBinary(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "gograph.exe")
	command := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/gograph")
	command.Dir = "../.."
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile CLI: %v\n%s", err, output)
	}
	return binary
}

func runSkillsCLI(t *testing.T, binary, root string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), 1
	}
	return string(output), 0
}

func skillsSymlinkFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	root := t.TempDir()
	writeExclusionFile(t, root, "go.mod", "module example.com/skills\n\ngo 1.27.0\n")
	writeExclusionFile(t, root, "main.go", "package skills\nfunc Target() {}\n")
	writeExclusionFile(t, root, "main_test.go", "package skills\nimport \"testing\"\nfunc TestTarget(t *testing.T) { Target() }\n")
	outside := t.TempDir()
	writeExclusionFile(t, outside, "SKILL.md", "# External skill\n")
	// A target which Go must never parse. Even an explicit import must see a
	// missing package, not this source or its diagnostic marker.
	writeExclusionFile(t, outside, "source.go", "package secret\ntype Secret SECRET_CONTENT_FROM_LINK_TARGET\n")
	if err := os.MkdirAll(filepath.Join(root, ".claude/skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".claude/skills/example")); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	return root
}

func TestExcludedSkillsLinksAreAbsentFromGoInputs(t *testing.T) {
	binary := skillsTestBinary(t)
	for _, exclusion := range []string{".claude", ".claude/skills"} {
		t.Run(exclusion, func(t *testing.T) {
			root := skillsSymlinkFixture(t)
			if _, code := runSkillsCLI(t, binary, root, "build", ".", "--precise", "--strict"); code == 0 {
				t.Fatal("unexcluded directory link unexpectedly accepted")
			}
			out, code := runSkillsCLI(t, binary, root, "build", ".", "--precise", "--strict", "--exclude-dirs="+exclusion)
			if code != 0 || strings.Contains(out, "warning:") {
				t.Fatalf("excluded skill blocked precise analysis: %d\n%s", code, out)
			}
			g, err := loadGraph(root)
			if err != nil || g.Build.EffectivePrecision() != graph.PrecisionPrecise || !g.Build.Complete || g.Build.TestCallResolution != graph.TestCallResolutionTyped || len(g.Files) != 2 {
				t.Fatalf("wrong selected graph: %+v, %v", g, err)
			}
		})
	}
}

func TestExcludedSkillsLinksCannotSupplyImportedGoCode(t *testing.T) {
	binary := skillsTestBinary(t)
	for _, scenario := range []string{"production", "test", "transitive", "replacement"} {
		t.Run(scenario, func(t *testing.T) {
			root := skillsSymlinkFixture(t)
			importLine := "import _ \"example.com/skills/.claude/skills/example\"\n"
			switch scenario {
			case "production":
				writeExclusionFile(t, root, "main.go", "package skills\n"+importLine+"func Target() {}\n")
			case "test":
				writeExclusionFile(t, root, "main_test.go", "package skills\n"+importLine)
			case "transitive":
				dependency := t.TempDir()
				writeExclusionFile(t, dependency, "go.mod", "module example.com/dependency\n\ngo 1.27.0\n")
				writeExclusionFile(t, dependency, "dependency.go", "package dependency\n"+importLine)
				writeExclusionFile(t, root, "go.mod", "module example.com/skills\n\ngo 1.27.0\nrequire example.com/dependency v0.0.0\nreplace example.com/dependency => "+filepath.ToSlash(dependency)+"\n")
				writeExclusionFile(t, root, "main.go", "package skills\nimport _ \"example.com/dependency\"\nfunc Target() {}\n")
			case "replacement":
				target, err := os.Readlink(filepath.Join(root, ".claude/skills/example"))
				if err != nil {
					t.Fatal(err)
				}
				writeExclusionFile(t, target, "go.mod", "module example.com/dependency\nSECRET_CONTENT_FROM_LINK_TARGET\n")
				writeExclusionFile(t, root, "go.mod", "module example.com/skills\n\ngo 1.27.0\nrequire example.com/dependency v0.0.0\nreplace example.com/dependency => ./.claude/skills/example\n")
				writeExclusionFile(t, root, "main.go", "package skills\nimport _ \"example.com/dependency\"\nfunc Target() {}\n")
			}
			stdout, code := runSkillsCLI(t, binary, root, "build", ".", "--precise", "--strict", "--exclude-dirs=.claude/skills")
			if strings.Contains(stdout, "SECRET_CONTENT_FROM_LINK_TARGET") {
				t.Fatalf("Go read the excluded link target: %s", stdout)
			}
			if scenario != "test" && code == 0 {
				t.Fatalf("linked dependency unexpectedly built: %s", stdout)
			}
			if scenario == "test" {
				g, err := loadGraph(root)
				if err != nil || g.Build.TestCallResolution == graph.TestCallResolutionTyped {
					t.Fatalf("linked test import silently accepted: %+v, %v", g, err)
				}
			}
		})
	}
}
