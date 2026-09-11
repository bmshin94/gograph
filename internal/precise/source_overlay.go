package precise

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ozgurcd/gograph/internal/buildctx"
)

// sourceLinkOverlay makes excluded directory links absent to cmd/go, including
// when reached by explicit or transitive imports. Skipping the safety walk alone
// would let Go follow those links. Never read or copy their target contents.
// The temporary filename is execution state, not persisted build identity.
func sourceLinkOverlay(ctx context.Context, root string, config buildctx.Config, links []string) ([]string, func(), error) {
	flags := config.Flags()
	cleanup := func() {}
	if len(links) == 0 {
		return flags, cleanup, nil
	}
	for _, flag := range flags {
		if strings.Contains(flag, "-overlay") {
			return nil, cleanup, fmt.Errorf("excluded directory symlinks cannot be combined with an existing Go -overlay")
		}
	}
	for _, env := range config.Environment() {
		if strings.HasPrefix(env, "GOFLAGS=") && strings.Contains(env, "-overlay") {
			return nil, cleanup, fmt.Errorf("excluded directory symlinks cannot be combined with GOFLAGS -overlay; remove the existing overlay")
		}
	}
	// GOFLAGS may also come from `go env -w`, not the process environment.
	// Do not silently discard an operator overlay from either source.
	command := exec.CommandContext(ctx, "go", "env", "GOFLAGS")
	command.Dir, command.Env = root, config.Environment()
	effective, err := command.Output()
	if err != nil {
		return nil, cleanup, fmt.Errorf("inspect effective Go overlay flags: %w", err)
	}
	if strings.Contains(string(effective), "-overlay") {
		return nil, cleanup, fmt.Errorf("excluded directory symlinks cannot be combined with an existing Go -overlay; remove it from GOFLAGS or GOENV")
	}
	replace := make(map[string]string, len(links))
	for _, path := range links {
		replace[path] = ""
	}
	file, err := os.CreateTemp("", "gograph-source-links-*.json")
	if err != nil {
		return nil, cleanup, err
	}
	cleanup = func() { _ = os.Remove(file.Name()) }
	err = json.NewEncoder(file).Encode(struct {
		Replace map[string]string
	}{replace})
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return append(flags, "-overlay="+file.Name()), cleanup, nil
}
