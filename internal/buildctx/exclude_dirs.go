package buildctx

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/ozgurcd/gograph/internal/sourcefs"
)

// NormalizeExcludeDirs canonicalizes literal repository-relative directory paths.
// Exclusions select analysis targets; they do not bypass source safety checks or
// prevent the Go toolchain from loading dependencies of included packages.
func NormalizeExcludeDirs(dirs []string) ([]string, error) {
	var normalized []string
	seen := make(map[string]bool)
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" || strings.HasPrefix(dir, "/") || strings.ContainsAny(dir, "\\:*?[]\x00\r\n,") {
			return nil, fmt.Errorf("invalid excluded directory %q: use a literal repository-relative directory", dir)
		}
		for _, part := range strings.Split(dir, "/") {
			if part == ".." {
				return nil, fmt.Errorf("excluded directory %q must stay beneath the repository root", dir)
			}
		}
		dir = path.Clean(dir)
		if dir == "." {
			return nil, fmt.Errorf("cannot exclude the repository root")
		}
		if !seen[dir] {
			seen[dir] = true
			normalized = append(normalized, dir)
		}
	}
	sort.Strings(normalized)
	// A parent exclusion subsumes its children. Equivalent selections must have
	// identical fingerprints regardless of spelling, order, or redundancy.
	var result []string
	for _, dir := range normalized {
		if !excludedPath(result, dir) {
			result = append(result, dir)
		}
	}
	return result, nil
}

func excludedPath(dirs []string, relative string) bool {
	for _, dir := range dirs {
		if relative == dir || strings.HasPrefix(relative, dir+"/") {
			return true
		}
	}
	return false
}

func (c Config) ExcludeDirs() []string { return append([]string(nil), c.excludeDirs...) }

func (c Config) Excludes(relative string) bool { return excludedPath(c.excludeDirs, relative) }

func (c Config) WithExcludeDirs(dirs []string) (Config, error) {
	var err error
	c.excludeDirs, err = NormalizeExcludeDirs(dirs)
	return c, err
}

// ValidateExcludeDirs rejects existing files and linked path components without
// following them. Missing directories are valid selections across branches.
func ValidateExcludeDirs(root string, dirs []string) error {
	normalized, err := NormalizeExcludeDirs(dirs)
	if err != nil || len(normalized) == 0 {
		return err
	}
	reader, err := sourcefs.Open(root)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	for _, dir := range normalized {
		if err := reader.ValidateDirectory(dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("excluded path %q must be a real repository directory: %w", dir, err)
		}
	}
	return nil
}
