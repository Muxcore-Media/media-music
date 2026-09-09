package internal_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Muxcore-Media/media-music/internal"
)

func openTempStore(t *testing.T) (*internal.Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "music.db")
	s, err := internal.OpenStore(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func TestStoreArtistAlbumRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, _ := openTempStore(t)
	ar, err := s.AddArtist(ctx, internal.Artist{Name: "Radiohead", Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	al, err := s.AddAlbum(ctx, internal.Album{ArtistID: ar.ID, Title: "OK Computer", Year: 1997, Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	if al.Title != "OK Computer" {
		t.Fatalf("%+v", al)
	}
	listed, err := s.ListAlbums(ctx, ar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("albums=%d", len(listed))
	}
	if err := s.RemoveArtist(ctx, ar.ID); err != nil {
		t.Fatal(err)
	}
	albums, err := s.ListAlbums(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 0 {
		t.Fatal("expected albums cleared with artist")
	}
}

func TestStoreRemoveArtistCascadesAlbumsTracksTags(t *testing.T) {
	ctx := context.Background()
	s, _ := openTempStore(t)
	ar, err := s.AddArtist(ctx, internal.Artist{Name: "Cascade Artist", Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	al, err := s.AddAlbum(ctx, internal.Album{ArtistID: ar.ID, Title: "Cascade Album", Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTrackFile(ctx, al.ID, filepath.Join(t.TempDir(), "track.mp3"), "Track", "FLAC", "mp3", 1); err != nil {
		t.Fatal(err)
	}
	tagID, err := s.CreateTag(ctx, "cascade-tag")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetItemTags(ctx, ar.ID, []string{tagID}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetItemTags(ctx, ar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != tagID || got[0].Label != "cascade-tag" {
		t.Fatalf("item tags: %+v", got)
	}
	if err := s.RemoveArtist(ctx, ar.ID); err != nil {
		t.Fatal(err)
	}
	albums, err := s.ListAlbums(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 0 {
		t.Fatalf("albums=%d", len(albums))
	}
	tracks, err := s.ListTracks(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 0 {
		t.Fatalf("tracks=%d", len(tracks))
	}
	tags, err := s.ListTags(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].ID != tagID {
		t.Fatalf("tag definition=%+v", tags)
	}
	filtered, _, err := s.ListArtistsPage(ctx, "", tagID, 1, 20, "name", "asc")
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 0 {
		t.Fatalf("item_tags should be cleared, artists=%d", len(filtered))
	}
	artists, err := s.ListArtists(ctx, "Cascade")
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 0 {
		t.Fatalf("artists=%d", len(artists))
	}
}

func TestStorePersistsAcrossOpen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "music.db")
	s1, err := internal.OpenStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	ar, err := s1.AddArtist(ctx, internal.Artist{Name: "Björk", Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s1.AddAlbum(ctx, internal.Album{ArtistID: ar.ID, Title: "Homogenic", Year: 1997}); err != nil {
		t.Fatal(err)
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := internal.OpenStore(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	artists, err := s2.ListArtists(ctx, "björk")
	if err != nil {
		t.Fatal(err)
	}
	if len(artists) != 1 {
		t.Fatalf("artists=%d", len(artists))
	}
	albums, err := s2.ListAlbums(ctx, artists[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 1 || albums[0].Title != "Homogenic" {
		t.Fatalf("%+v", albums)
	}
}
