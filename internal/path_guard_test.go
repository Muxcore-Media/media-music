package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathAllowedRejectsSymlinkEscape(t *testing.T) {
	lib := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(lib, "escape.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	m := &Module{libraryDir: lib}
	if m.pathAllowed(link) {
		t.Fatal("symlink escape was allowed")
	}
	real := filepath.Join(lib, "ok.mp3")
	if err := os.WriteFile(real, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !m.pathAllowed(real) {
		t.Fatal("file inside library_dir was refused")
	}
	if m.pathAllowed(secret) {
		t.Fatal("path outside library_dir was allowed")
	}
}
