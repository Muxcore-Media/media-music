package internal

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Muxcore-Media/core/sdk/go/module/pathguard"
)

func (m *Module) libraryRoots() []string {
	root := strings.TrimSpace(m.getLibraryDir())
	if root == "" {
		return nil
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	return []string{abs}
}

func (m *Module) pathAllowed(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	roots := m.libraryRoots()
	if len(roots) == 0 {
		return false
	}
	if _, err := pathguard.Confine(abs, roots); err != nil {
		return false
	}
	return true
}

func (m *Module) requirePathAllowed(ctx context.Context, path, artistID string) error {
	if m.pathAllowed(path) {
		return nil
	}
	_ = ctx
	_ = artistID
	return fmt.Errorf("path %q is outside library_dir", path)
}
