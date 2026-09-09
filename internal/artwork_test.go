package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mediaadminv1 "github.com/Muxcore-Media/contracts-media-admin/gen/muxcore/media/admin/v1"
)

func TestWriteAndListArtistArtwork(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	m := NewModule(Config{DataDir: data, LibraryDir: filepath.Join(data, "music"), GRPCAddr: "127.0.0.1:0", HTTPAddr: "127.0.0.1:0"})
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(ctx) })

	ar, err := m.store.AddArtist(ctx, Artist{Name: "Poster Artist", Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	rel, _, err := m.writeArtworkBytes(ar.ID, "cover.jpg", "image/jpeg", []byte("fakepng"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.UpdateArtist(ctx, ar.ID, map[string]any{"poster_url": rel}); err != nil {
		t.Fatal(err)
	}
	resp, err := m.ListArtwork(ctx, &mediaadminv1.ListArtworkRequest{Id: ar.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetArtwork()) != 1 || !strings.Contains(resp.GetArtwork()[0].GetUrl(), "/images/"+ar.ID+"/poster.jpg") {
		t.Fatalf("artwork=%+v", resp.GetArtwork())
	}
	if _, err := os.Stat(filepath.Join(m.getImageDir(), filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
}
