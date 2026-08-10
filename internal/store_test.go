package internal_test

import (
	"testing"

	"github.com/Muxcore-Media/media-music/internal"
)

func TestStoreArtistAlbumRoundTrip(t *testing.T) {
	s := internal.NewStore()
	ar, err := s.AddArtist(internal.Artist{Name: "Radiohead", Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	al, err := s.AddAlbum(internal.Album{ArtistID: ar.ID, Title: "OK Computer", Year: 1997, Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	if al.Title != "OK Computer" {
		t.Fatalf("%+v", al)
	}
	listed := s.ListAlbums(ar.ID)
	if len(listed) != 1 {
		t.Fatalf("albums=%d", len(listed))
	}
	if err := s.RemoveArtist(ar.ID); err != nil {
		t.Fatal(err)
	}
	if len(s.ListAlbums("")) != 0 {
		t.Fatal("expected albums cleared with artist")
	}
}
