package scanner

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ozgurcd/gograph/internal/buildctx"
)

// ExcludedDirectoryLinks validates the Go source trees and returns directory
// symlinks beneath explicitly excluded directories. These links may ONLY be
// allowed when the caller masks them from the Go tool using a deletion overlay.
// This function never enumerates or reads a linked directory's contents.
// Linked Go files and metadata remain errors, even within exclusions.
func ExcludedDirectoryLinks(root string, exclusions []string) ([]string, error) {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return nil, err
	}
	exclusions, err = buildctx.NormalizeExcludeDirs(exclusions)
	if err != nil {
		return nil, err
	}
	if err := buildctx.ValidateExcludeDirs(canonical, exclusions); err != nil {
		return nil, err
	}
	links := make(map[string]bool)
	err = validateToolchainLinks(root, func(path string) bool {
		rel, err := filepath.Rel(canonical, path)
		if err != nil || !filepath.IsLocal(rel) {
			return false
		}
		rel = filepath.ToSlash(rel)
		for _, dir := range exclusions {
			if strings.HasPrefix(rel, dir+"/") {
				links[path] = true
				return true
			}
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(links))
	for path := range links {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

// ValidateMaskedSourceLinks rechecks only the exact link paths that the caller
// already hid from Go. A newly introduced link is not silently authorized.
func ValidateMaskedSourceLinks(root string, masked []string) error {
	allowed := make(map[string]bool, len(masked))
	for _, path := range masked {
		allowed[path] = true
	}
	return validateToolchainLinks(root, func(path string) bool { return allowed[path] })
}

func validateToolchainLinks(root string, mask func(string) bool) error {
	roots, err := buildctx.ToolchainSourceRoots(root)
	if err != nil {
		return fmt.Errorf("validate Go tool metadata: %w", err)
	}
	roots = append([]string{root}, roots...)
	seen := make(map[string]bool)
	for _, sourceRoot := range roots {
		canonical, err := filepath.EvalSymlinks(sourceRoot)
		if err != nil {
			return err
		}
		canonical, err = filepath.Abs(canonical)
		if err != nil {
			return err
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		if err := validateNoSourceLinks(canonical, mask); err != nil {
			return fmt.Errorf("validate Go tool source tree %s: %w", sourceRoot, err)
		}
	}
	return nil
}
