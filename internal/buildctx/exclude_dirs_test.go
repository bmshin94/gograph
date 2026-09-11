package buildctx

import (
	"context"
	"go/build"
	"reflect"
	"testing"
)

func TestNormalizeExcludeDirs(t *testing.T) {
	got, err := NormalizeExcludeDirs([]string{"./legacy/", "vendor-old", "legacy/nested", "legacy", " other "})
	if err != nil || !reflect.DeepEqual(got, []string{"legacy", "other", "vendor-old"}) {
		t.Fatalf("normalized=%v err=%v", got, err)
	}
	for _, invalid := range []string{"", ".", "./", "..", "../outside", "a/../b", "/tmp", "C:/tmp", `a\b`, "a/*", "a,b", "a\n"} {
		// Leading/trailing whitespace is allowed, but embedded newlines are not.
		if invalid == "a\n" {
			invalid = "a\nb"
		}
		if _, err := NormalizeExcludeDirs([]string{invalid}); err == nil {
			t.Errorf("accepted invalid directory %q", invalid)
		}
	}
}

func TestExcludeDirsIdentityAndFallback(t *testing.T) {
	base := FromBuildContext(build.Default, nil)
	selected, err := base.WithExcludeDirs([]string{"./bad/", "bad/sub"})
	if err != nil {
		t.Fatal(err)
	}
	if base.Fingerprint() == selected.Fingerprint() {
		t.Fatal("exclusions not fingerprinted")
	}
	equivalent, _ := base.WithExcludeDirs([]string{"bad"})
	if equivalent.Fingerprint() != selected.Fingerprint() {
		t.Fatal("equivalent exclusions differ")
	}
	if !selected.Excludes("bad/file.go") || selected.Excludes("bad-other/file.go") {
		t.Fatal("incorrect subtree matching")
	}
	copy := selected.ExcludeDirs()
	copy[0] = "changed"
	if selected.ExcludeDirs()[0] != "bad" {
		t.Fatal("mutable exclusion storage exposed")
	}
	if selected.WithBuildContext(build.Default).Fingerprint() != selected.Fingerprint() {
		t.Fatal("selection replay drops exclusions")
	}
	fallback, err := ResolveOrDefaultWithOptions(context.Background(), t.TempDir()+"/missing", ResolveOptions{ExcludeDirs: []string{"bad"}})
	if err == nil || !fallback.Excludes("bad/file.go") {
		t.Fatalf("fallback lost exclusion: %v %v", fallback.ExcludeDirs(), err)
	}
}
