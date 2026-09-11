package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ozgurcd/gograph/internal/graph"
	"github.com/ozgurcd/gograph/internal/search"
	"github.com/ozgurcd/gograph/internal/validation"
	workspacegraph "github.com/ozgurcd/gograph/internal/workspace"
)

func TestExcludeDirsBuildMCPArgumentParity(t *testing.T) {
	for _, args := range [][]string{
		{"--exclude-dirs=bad,legacy/nested", ".", "--exclude-dirs", "legacy"},
		{".", "--exclude-dirs", "./legacy/,bad"},
	} {
		build, err := parseBuildArgs(args)
		if err != nil {
			t.Fatal(err)
		}
		mcp, err := parseMCPArgs(args)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(build.ExcludeDirs, []string{"bad", "legacy"}) || !reflect.DeepEqual(build.ExcludeDirs, mcp.ExcludeDirs) {
			t.Fatalf("build=%v mcp=%v", build.ExcludeDirs, mcp.ExcludeDirs)
		}
	}
	for _, arg := range []string{"--exclude-dirs", "--exclude-dirs=", "--exclude-dirs=bad,", "--exclude-dirs=../bad", "--exclude-dirs=/tmp", "--exclude-dirs=."} {
		if _, err := parseBuildArgs([]string{arg}); err == nil {
			t.Errorf("build accepted %q", arg)
		}
		if _, err := parseMCPArgs([]string{arg}); err == nil {
			t.Errorf("MCP accepted %q", arg)
		}
	}
}

func exclusionFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("GOWORK", "off")
	t.Setenv("GOFLAGS", "")
	root := t.TempDir()
	writeExclusionFile(t, root, "go.mod", "module example.com/exclusions\n\ngo 1.27.0\n")
	writeExclusionFile(t, root, "good/good.go", "package good\nfunc Target() {}\nfunc Caller() { Target() }\n")
	writeExclusionFile(t, root, "good/good_test.go", "package good\nimport \"testing\"\nfunc TestTarget(t *testing.T) { Target() }\n")
	writeExclusionFile(t, root, "broken/broken.go", "package broken\nfunc Broken() { undefined() }\n")
	writeExclusionFile(t, root, "broken/broken_test.go", "package broken\nfunc TestBroken(t *missing.T) {}\n")
	writeExclusionFile(t, root, "broken-other/other.go", "package other\nfunc KeepSibling() {}\n")
	return root
}

func writeExclusionFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExcludeDirsRepairsPreciseBuildAndTracksSelection(t *testing.T) {
	root := exclusionFixture(t)
	if rc := runBuild([]string{root, "--precise", "--strict"}); rc == 0 {
		t.Fatal("broken package unexpectedly precise")
	}
	if rc := runBuild([]string{root, "--precise", "--strict", "--exclude-dirs=broken"}); rc != 0 {
		t.Fatal("excluded broken package still blocks precise build")
	}
	g, err := loadGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if g.Build.EffectivePrecision() != graph.PrecisionPrecise || !g.Build.Complete || g.Build.TestCallResolution != graph.TestCallResolutionTyped {
		t.Fatalf("unexpected build metadata: %+v", g.Build)
	}
	if !reflect.DeepEqual(g.Build.Selection.ExcludeDirs, []string{"broken"}) {
		t.Fatal("missing recorded exclusions")
	}
	for _, f := range g.Files {
		if strings.HasPrefix(filepath.ToSlash(f.Path), "broken/") {
			t.Fatalf("excluded file leaked: %s", f.Path)
		}
	}
	if len(search.Query(g, []string{"KeepSibling"})) == 0 {
		t.Fatal("prefix sibling wrongly excluded")
	}
	if sr := search.Stale(g, root); sr.IsStale {
		t.Fatalf("new graph stale: %+v", sr)
	}
	diagnostic, findings := inspectDoctorRepository(root, nil)
	if diagnostic == nil || diagnostic.Freshness != "current" || len(findings) != 0 || !reflect.DeepEqual(diagnostic.ExcludeDirs, []string{"broken"}) {
		t.Fatalf("doctor disagrees with selected graph: %+v %+v", diagnostic, findings)
	}
	if _, err := (validation.RepositoryLoader{}).Load(context.Background(), root); err == nil {
		t.Fatal("full-repository machine validation silently accepted excluded selection")
	}
	writeExclusionFile(t, root, "broken/new.go", "this is not Go syntax")
	if sr := search.Stale(g, root); sr.IsStale {
		t.Fatalf("excluded edit invalidated graph: %+v", sr)
	}
	if rc := runBuild([]string{root, "--precise", "--strict", "--exclude-dirs=broken"}); rc != 0 {
		t.Fatal("syntax errors in excluded directory still block precise build")
	}
	without, _ := resolveBuildConfigWithTags(root, nil)
	if !search.StaleWithConfig(g, root, without).BuildContextChanged {
		t.Fatal("removing exclusions did not invalidate selection")
	}
	writeExclusionFile(t, root, "good/good.go", "package good\nfunc Target() {}\nfunc Caller() { Target() }\nfunc Added() {}\n")
	if !search.Stale(g, root).IsStale {
		t.Fatal("included edit not detected")
	}
}

func TestExcludeDirsDoesNotHideImportedDependencyFailure(t *testing.T) {
	root := exclusionFixture(t)
	writeExclusionFile(t, root, "good/import.go", "package good\nimport \"example.com/exclusions/broken\"\nfunc UseBroken() { broken.Broken() }\n")
	if rc := runBuild([]string{root, "--precise", "--strict", "--exclude-dirs=broken"}); rc == 0 {
		t.Fatal("excluded imported dependency error was hidden")
	}
	g, err := loadGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if g.Build.EffectivePrecision() != graph.PrecisionFallback {
		t.Fatalf("precision=%s", g.Build.EffectivePrecision())
	}
}

func TestExcludeDirsActualMCPRefreshParity(t *testing.T) {
	t.Run("matching startup selection", func(t *testing.T) { testExcludeDirsActualMCP(t, true) })
	t.Run("omitted startup exclusions", func(t *testing.T) { testExcludeDirsActualMCP(t, false) })
}

func testExcludeDirsActualMCP(t *testing.T, passExclusions bool) {
	root := exclusionFixture(t)
	if rc := runBuild([]string{root, "--precise", "--strict", "--exclude-dirs=broken"}); rc != 0 {
		t.Fatal("build failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "gograph.exe")
	compile := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/gograph")
	compile.Dir = "../.."
	if output, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("build current MCP executable: %v\n%s", err, output)
	}
	for _, verb := range []string{"build", "mcp", "stale"} {
		output, err := exec.CommandContext(ctx, binary, verb, "--help").CombinedOutput()
		if err != nil || !strings.Contains(string(output), "--exclude-dirs=dir1,dir2") || !strings.Contains(string(output), "Imported dependencies") {
			t.Fatalf("%s help missing exclusion contract: %v\n%s", verb, err, output)
		}
	}
	build := exec.CommandContext(ctx, binary, "build", root, "--precise", "--strict", "--exclude-dirs=broken")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("actual CLI build failed: %v\n%s", err, output)
	}
	before, err := os.ReadFile(filepath.Join(root, graphFile))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"mcp", root}
	if passExclusions {
		args = append(args, "--exclude-dirs=broken")
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = in.Close(); cancel(); _ = cmd.Wait() })
	encoder, decoder := json.NewEncoder(in), json.NewDecoder(out)
	id := 0
	request := func(method string, params any) json.RawMessage {
		t.Helper()
		id++
		if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
			t.Fatal(err)
		}
		var response struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  any             `json:"error"`
		}
		if err := decoder.Decode(&response); err != nil {
			t.Fatal(err)
		}
		if response.ID != id || response.Error != nil {
			t.Fatalf("unexpected response: %+v", response)
		}
		return response.Result
	}
	request("initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "exclusion-test", "version": "1"}})
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		t.Fatal(err)
	}
	call := func(name string, arguments map[string]any) string {
		t.Helper()
		result := request("tools/call", map[string]any{"name": name, "arguments": arguments})
		var payload struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(result, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.IsError || len(payload.Content) == 0 {
			t.Fatalf("tool failed: %s", result)
		}
		return payload.Content[0].Text
	}
	capabilities := call("gograph_capabilities", map[string]any{})
	var caps map[string]any
	if err := json.Unmarshal([]byte(capabilities), &caps); err != nil {
		t.Fatal(err)
	}
	selection := caps["analysis_build_context"].(map[string]any)
	if !passExclusions {
		if fmt.Sprint(selection["exclude_dirs"]) != "[]" {
			t.Fatalf("silently inherited exclusions: %v", selection)
		}
		var stale search.StaleResult
		if err := json.Unmarshal([]byte(call("gograph_stale", map[string]any{})), &stale); err != nil {
			t.Fatal(err)
		}
		if !stale.IsStale || !stale.BuildContextChanged {
			t.Fatalf("MCP did not detect differing startup selection: %+v", stale)
		}
		return
	}
	if fmt.Sprint(selection["exclude_dirs"]) != "[broken]" {
		t.Fatalf("wrong MCP exclusions: %v", selection)
	}
	if result := call("gograph_query", map[string]any{"term": "Broken"}); strings.Contains(result, `"name": "Broken"`) {
		t.Fatalf("excluded symbol leaked: %s", result)
	}
	writeExclusionFile(t, root, "good/good.go", "package good\nfunc Target() {}\nfunc Caller() { Target() }\nfunc FreshAfterRefresh() {}\n")
	if result := call("gograph_query", map[string]any{"term": "FreshAfterRefresh"}); !strings.Contains(result, "FreshAfterRefresh") {
		t.Fatalf("refresh missed included change: %s", result)
	}
	after, err := os.ReadFile(filepath.Join(root, graphFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("read-only MCP refresh changed persisted artifact")
	}
}

func TestExcludeDirsPreservesSourceSafety(t *testing.T) {
	root := exclusionFixture(t)
	external := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(external, []byte("package broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "broken", "linked.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if rc := runBuild([]string{root, "--precise", "--strict", "--exclude-dirs=broken"}); rc == 0 {
		t.Fatal("exclusions bypassed source safety validation")
	}
}

func TestExcludeDirsRejectsIndividualFileWithoutPublishing(t *testing.T) {
	root := exclusionFixture(t)
	if rc := runBuild([]string{root, "--exclude-dirs=good/good.go"}); rc == 0 {
		t.Fatal("directory flag accepted a file")
	}
	if _, err := os.Stat(filepath.Join(root, graphFile)); !os.IsNotExist(err) {
		t.Fatalf("invalid exclusion published graph: %v", err)
	}
	if _, _, _, err := prepareMCPGraphWithState(mcpOptions{Root: root, ExcludeDirs: []string{"good/good.go"}}); err == nil {
		t.Fatal("MCP accepted a file exclusion")
	}
}

func TestExcludeDirsWorkspaceRefreshAndQuery(t *testing.T) {
	root := exclusionFixture(t)
	workspaceRoot := filepath.Dir(root)
	manifest := fmt.Sprintf("schema_version: gograph.workspace-manifest.v1\nname: test\nrepositories:\n  - id: selected\n    path: %s\n    precision: precise\n    exclude_dirs: [broken]\n", filepath.Base(root))
	writeExclusionFile(t, workspaceRoot, workspacegraph.ManifestFile, manifest)
	if rc := runWorkspaceBuild([]string{workspaceRoot, "--refresh-members"}); rc != 0 {
		t.Fatal("workspace refresh failed")
	}
	loaded, err := workspacegraph.Load(context.Background(), workspaceRoot)
	if err != nil || loaded == nil {
		t.Fatalf("workspace CLI/MCP query loader failed: %v", err)
	}
	configuration, _, err := workspacegraph.LoadManifest(workspaceRoot)
	if err != nil {
		t.Fatal(err)
	}
	inspection := workspacegraph.InspectMember(context.Background(), workspaceRoot, configuration.Repositories[0])
	if inspection.Error != nil || inspection.Loaded == nil {
		t.Fatalf("excluded member not usable: %+v", inspection)
	}
	writeExclusionFile(t, workspaceRoot, workspacegraph.ManifestFile, strings.ReplaceAll(manifest, "exclude_dirs: [broken]", "exclude_dirs: [broken, good]"))
	if _, err := workspacegraph.Load(context.Background(), workspaceRoot); err == nil {
		t.Fatal("changed exclusions did not invalidate workspace")
	}
}
