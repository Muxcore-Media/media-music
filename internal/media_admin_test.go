package internal_test

import (
	"context"
	"path/filepath"
	"testing"

	automationv1 "github.com/Muxcore-Media/contracts-automation/muxcore/automation/v1"
	mediaadminv1 "github.com/Muxcore-Media/contracts-media-admin/gen/muxcore/media/admin/v1"
	"github.com/Muxcore-Media/media-music/internal"
)

func TestMediaAdminListMissingTagsHistory(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	lib := filepath.Join(data, "music")
	m := internal.NewModule(internal.Config{DataDir: data, LibraryDir: lib, GRPCAddr: "127.0.0.1:0", HTTPAddr: "127.0.0.1:0"})
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(ctx) })

	ar, err := m.Store().AddArtist(ctx, internal.Artist{Name: "Tagged Artist", Monitored: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Store().AddAlbum(ctx, internal.Album{ArtistID: ar.ID, Title: "Missing Album", Monitored: true}); err != nil {
		t.Fatal(err)
	}
	tagID, err := m.Store().CreateTag(ctx, "rock")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Store().SetItemTags(ctx, ar.ID, []string{tagID}); err != nil {
		t.Fatal(err)
	}
	m.Store().AppendHistory(ctx, ar.ID, "import", ar.Name, "")

	missing, err := m.ListMissingAdmin(ctx, &mediaadminv1.ListMissingRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing.GetItems()) != 1 {
		t.Fatalf("missing=%d", len(missing.GetItems()))
	}

	tags, err := m.ListTagsAdmin(ctx, &mediaadminv1.ListTagsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tags.GetTags()) != 1 || tags.GetTags()[0].GetLabel() != "rock" {
		t.Fatalf("tags=%+v", tags.GetTags())
	}

	history, err := m.ListHistoryAdmin(ctx, &mediaadminv1.ListHistoryRequest{ItemId: ar.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.GetRecords()) != 1 {
		t.Fatalf("history=%d", len(history.GetRecords()))
	}
}

func TestSearchIndexersFixtureMatches(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	lib := filepath.Join(data, "music")
	m := internal.NewModule(internal.Config{DataDir: data, LibraryDir: lib, GRPCAddr: "127.0.0.1:0", HTTPAddr: "127.0.0.1:0"})
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(ctx) })

	ar, err := m.Store().AddArtist(ctx, internal.Artist{Name: "Indexer Artist", Monitored: true, QualityProfileID: "qp1"})
	if err != nil {
		t.Fatal(err)
	}
	m.SetAutomationSearchOverride(func(_ context.Context, req *automationv1.SearchItemRequest) (*automationv1.SearchItemResponse, error) {
		if req.GetQuery() != "Indexer Artist" {
			t.Fatalf("query=%q", req.GetQuery())
		}
		return &automationv1.SearchItemResponse{
			Matches: []*automationv1.ReleaseMatch{{Title: "Indexer Artist - FLAC", Guid: "g1", IndexerName: "fixture"}},
		}, nil
	})

	resp, err := m.SearchIndexers(ctx, &mediaadminv1.SearchIndexersRequest{ItemId: ar.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetResults()) != 1 {
		t.Fatalf("results=%d", len(resp.GetResults()))
	}
	if resp.GetResults()[0].GetQuality() != "FLAC" {
		t.Fatalf("quality=%q", resp.GetResults()[0].GetQuality())
	}

	m.SetAutomationSearchOverride(func(context.Context, *automationv1.SearchItemRequest) (*automationv1.SearchItemResponse, error) {
		return &automationv1.SearchItemResponse{}, nil
	})
	empty, err := m.SearchIndexers(ctx, &mediaadminv1.SearchIndexersRequest{ItemId: ar.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.GetResults()) != 0 {
		t.Fatalf("expected empty results, got %d", len(empty.GetResults()))
	}
}
