package internal_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Muxcore-Media/media-music/internal"
)

// TestScanLibraryRootFixtures walks local stub audio files under testdata/library.
// Files are empty stubs (no paid metadata APIs).
func TestScanLibraryRootFixtures(t *testing.T) {
	root := filepath.Join("testdata", "library")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("fixture library missing: %v", err)
	}

	s, _ := openTempStore(t)
	res, err := s.ScanLibraryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesFound < 2 {
		t.Fatalf("expected >=2 audio stubs, found=%d imported=%d skipped=%d",
			res.FilesFound, res.FilesImported, res.FilesSkipped)
	}
	if res.FilesImported < 2 {
		t.Fatalf("expected imports, got %+v", res)
	}

	artists, err := s.ListArtists("")
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) < 1 {
		t.Fatal("expected at least one artist from fixtures")
	}
	var fixtureArtist *internal.Artist
	for _, a := range artists {
		if a.Name == "Fixture Artist" {
			fixtureArtist = a
			break
		}
	}
	if fixtureArtist == nil {
		t.Fatalf("Fixture Artist not found: %+v", artists)
	}
	albums, err := s.ListAlbums(fixtureArtist.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) < 1 {
		t.Fatal("expected album from fixtures")
	}
	tracks, err := s.ListTracks(albums[0].ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) < 2 {
		t.Fatalf("expected mp3+flac stubs, tracks=%d", len(tracks))
	}

	// Idempotent rescan should skip already-imported paths.
	res2, err := s.ScanLibraryRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if res2.FilesImported != 0 {
		t.Fatalf("expected no new imports on rescan, got %+v", res2)
	}
	if res2.FilesSkipped < 2 {
		t.Fatalf("expected skips on rescan, got %+v", res2)
	}
}

func TestScanLibraryRootMissing(t *testing.T) {
	s, _ := openTempStore(t)
	_, err := s.ScanLibraryRoot(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing root")
	}
}

func TestModuleInitScan(t *testing.T) {
	data := t.TempDir()
	lib := filepath.Join(data, "music")
	src := filepath.Join("testdata", "library")
	if err := copyTree(src, lib); err != nil {
		t.Fatal(err)
	}

	m := internal.NewModule(internal.Config{
		DataDir:    data,
		LibraryDir: lib,
		GRPCAddr:   "127.0.0.1:0",
		HTTPAddr:   "127.0.0.1:0",
	})
	if err := m.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(t.Context()) })

	res, err := m.ScanLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesImported < 2 {
		t.Fatalf("%+v", res)
	}
}

// TestScanConfiguredLibrary scans MUSIC_DATA_DIR / MUSIC_LIBRARY_DIR when RUN_LIBRARY_SCAN=1.
func TestScanConfiguredLibrary(t *testing.T) {
	if os.Getenv("RUN_LIBRARY_SCAN") != "1" {
		t.Skip("set RUN_LIBRARY_SCAN=1")
	}
	m := internal.NewModule(internal.Config{
		GRPCAddr: "127.0.0.1:0",
		HTTPAddr: "127.0.0.1:0",
	})
	if err := m.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(t.Context()) })
	res, err := m.ScanLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesFound == 0 {
		t.Fatalf("no files found in library root: %+v", res)
	}
	t.Logf("scan: %+v", res)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
