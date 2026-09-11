package precise

import (
	"context"
	"encoding/json"
	"go/build"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ozgurcd/gograph/internal/buildctx"
)

func TestSourceLinkOverlayDeletionAndCleanup(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	config := buildctx.FromBuildContext(build.Default, os.Environ())
	link := filepath.Join(t.TempDir(), "skills", "example")
	flags, cleanup, err := sourceLinkOverlay(context.Background(), t.TempDir(), config, []string{link})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	path := strings.TrimPrefix(flags[len(flags)-1], "-overlay=")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var overlay struct{ Replace map[string]string }
	if err := json.Unmarshal(data, &overlay); err != nil || len(overlay.Replace) != 1 {
		t.Fatalf("bad overlay: %s, %v", data, err)
	}
	if value, found := overlay.Replace[link]; !found || value != "" {
		t.Fatalf("link not deleted: %v", overlay.Replace)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary overlay retained: %v", err)
	}
}

func TestSourceLinkOverlayDoesNotOverrideUserOverlay(t *testing.T) {
	config := buildctx.FromBuildContext(build.Default, []string{"GOFLAGS=-overlay=/operator/overlay.json"})
	if _, _, err := sourceLinkOverlay(context.Background(), t.TempDir(), config, []string{"/excluded/skill"}); err == nil {
		t.Fatal("silently replaced operator overlay")
	}
	flags, cleanup, err := sourceLinkOverlay(context.Background(), t.TempDir(), config, nil)
	if err != nil || !reflect.DeepEqual(flags, config.Flags()) {
		t.Fatalf("changed configuration without masked links: %v, %v", flags, err)
	}
	cleanup()
}

func TestSourceLinkOverlayHonorsPersistedGoEnv(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, "goenv")
	if err := os.WriteFile(envPath, []byte("GOFLAGS=-overlay=/operator/overlay.json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", envPath)
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "off")
	config := buildctx.FromBuildContext(build.Default, os.Environ())
	_, _, err := sourceLinkOverlay(context.Background(), root, config, []string{filepath.Join(root, "skills", "example")})
	if err == nil || !strings.Contains(err.Error(), "Go -overlay") {
		t.Fatalf("persisted operator overlay not protected: %v", err)
	}
}
