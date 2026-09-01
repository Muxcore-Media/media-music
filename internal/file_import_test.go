package internal_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Muxcore-Media/core/pkg/contracts"
	"github.com/Muxcore-Media/media-music/internal"
)

func TestHandleMusicFileImportedUpsertsLibrary(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	lib := filepath.Join(data, "music")
	if err := copyTree(filepath.Join("testdata", "library"), lib); err != nil {
		t.Fatal(err)
	}
	trackPath := filepath.Join(lib, "Fixture Artist", "Sample Album", "01 - Intro.mp3")

	m := internal.NewModule(internal.Config{DataDir: data, LibraryDir: lib, GRPCAddr: "127.0.0.1:0", HTTPAddr: "127.0.0.1:0"})
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(ctx) })

	if err := m.HandleMusicFileImported(ctx, contracts.FileImportedPayload{
		MediaType:       "music",
		Title:           "Intro",
		DestinationPath: trackPath,
		Quality:         "FLAC",
	}); err != nil {
		t.Fatal(err)
	}

	artists, err := m.Store().ListArtists(ctx, "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 1 {
		t.Fatalf("artists=%d", len(artists))
	}
	albums, err := m.Store().ListAlbums(ctx, artists[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 1 {
		t.Fatalf("albums=%d", len(albums))
	}
	tracks, err := m.Store().ListTracks(ctx, albums[0].ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 {
		t.Fatalf("tracks=%+v", tracks)
	}
	if tracks[0].Title != "Intro" {
		t.Fatalf("title=%q", tracks[0].Title)
	}
	st, err := os.Stat(trackPath)
	if err != nil {
		t.Fatal(err)
	}
	if tracks[0].Path != trackPath {
		t.Fatalf("path=%q", tracks[0].Path)
	}
	_ = st
}
