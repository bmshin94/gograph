package workspace

import (
	"fmt"

	"github.com/ozgurcd/gograph/internal/buildctx"
)

func normalizeRepositoryExclusions(repo *RepositoryConfig) error {
	dirs, err := buildctx.NormalizeExcludeDirs(repo.ExcludeDirs)
	if err != nil {
		return fmt.Errorf("repository %q exclude_dirs: %w", repo.ID, err)
	}
	repo.ExcludeDirs = dirs
	return nil
}
