package internal

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func isUnderRoot(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	pathAbs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	if pathAbs == rootAbs {
		return true
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (m *Module) pathAllowed(path string) bool {
	if path == "" {
		return false
	}
	if isUnderRoot(path, m.getLibraryDir()) {
		return true
	}
	return false
}

func (m *Module) pathAllowedForArtist(ctx context.Context, path, artistID string) bool {
	if m.pathAllowed(path) {
		return true
	}
	if artistID == "" || m.store == nil {
		return false
	}
	a, err := m.store.GetArtist(ctx, artistID)
	if err != nil || strings.TrimSpace(a.Path) == "" {
		return false
	}
	return isUnderRoot(path, a.Path)
}

func (m *Module) requirePathAllowed(ctx context.Context, path, artistID string) error {
	if m.pathAllowedForArtist(ctx, path, artistID) {
		return nil
	}
	return fmt.Errorf("path %q is outside library_dir", path)
}
