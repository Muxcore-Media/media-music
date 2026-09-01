package internal_test

import (
	"context"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

	musicbrainzv1 "github.com/Muxcore-Media/metadata-musicbrainz/proto/musicbrainzv1"
	"github.com/Muxcore-Media/media-music/internal"
)

type fakeMusicBrainz struct {
	searchResp *musicbrainzv1.SearchArtistResponse
	getResp    *musicbrainzv1.GetArtistResponse
}

func (f *fakeMusicBrainz) SearchArtist(_ context.Context, _ *musicbrainzv1.SearchArtistRequest, _ ...grpc.CallOption) (*musicbrainzv1.SearchArtistResponse, error) {
	return f.searchResp, nil
}

func (f *fakeMusicBrainz) GetArtist(_ context.Context, _ *musicbrainzv1.GetArtistRequest, _ ...grpc.CallOption) (*musicbrainzv1.GetArtistResponse, error) {
	return f.getResp, nil
}

func TestSyncArtistMetadataResolvesMBID(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	lib := filepath.Join(data, "music")
	m := internal.NewModule(internal.Config{DataDir: data, LibraryDir: lib, GRPCAddr: "127.0.0.1:0", HTTPAddr: "127.0.0.1:0"})
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(ctx) })

	fake := &fakeMusicBrainz{
		searchResp: &musicbrainzv1.SearchArtistResponse{
			Artists: []*musicbrainzv1.ArtistSearchResult{{Id: "mbid-fixture", Name: "Radiohead", Score: 100}},
		},
		getResp: &musicbrainzv1.GetArtistResponse{
			Id: "mbid-fixture", Name: "Radiohead", Disambiguation: "English rock band",
			ReleaseGroups: []*musicbrainzv1.ReleaseGroup{
				{Id: "rg-1", Title: "OK Computer", PrimaryType: "Album", FirstReleaseDateYear: 1997},
			},
		},
	}
	m.SetMusicBrainzOverride(fake)

	ar, err := m.Store().AddArtist(ctx, internal.Artist{Name: "Radiohead", Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.SyncArtistMetadata(ctx, ar); err != nil {
		t.Fatal(err)
	}
	updated, err := m.Store().GetArtist(ctx, ar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.MusicBrainzID != "mbid-fixture" {
		t.Fatalf("mbid=%q", updated.MusicBrainzID)
	}
	albums, err := m.Store().ListAlbums(ctx, ar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 1 || albums[0].Title != "OK Computer" {
		t.Fatalf("albums=%+v", albums)
	}
}
