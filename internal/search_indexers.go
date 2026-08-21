package internal

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	mediaadminv1 "github.com/Muxcore-Media/contracts-media-admin/gen/muxcore/media/admin/v1"
	automationv1 "github.com/Muxcore-Media/media-automation/proto/automationv1"
)

const capMediaAutomation = "media.automation"

func (m *Module) SearchIndexers(ctx context.Context, req *mediaadminv1.SearchIndexersRequest) (*mediaadminv1.SearchIndexersResponse, error) {
	if req.GetItemId() == "" {
		return nil, status.Error(codes.InvalidArgument, "item_id required")
	}
	a, err := mustArtist(m.store, req.GetItemId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	limit := req.GetLimit()
	if limit <= 0 {
		limit = 50
	}
	searchResp, err := m.automationSearchItem(ctx, &automationv1.SearchItemRequest{
		ItemType: "music", Query: a.Name, Limit: limit, QualityProfileId: a.QualityProfileID,
	})
	if err != nil {
		return &mediaadminv1.SearchIndexersResponse{}, nil
	}
	results := releaseMatchesToIndexerResults(searchResp.GetMatches())
	return &mediaadminv1.SearchIndexersResponse{Results: results, Total: int32(len(results))}, nil
}

func (m *Module) automationSearchItem(ctx context.Context, req *automationv1.SearchItemRequest) (*automationv1.SearchItemResponse, error) {
	if m.automationSearchFn != nil {
		return m.automationSearchFn(ctx, req)
	}
	if m.mc == nil {
		return nil, fmt.Errorf("not connected to core")
	}
	addr, err := m.findAutomationAddr(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return automationv1.NewAutomationServiceClient(conn).SearchItem(ctx, req)
}

func (m *Module) findAutomationAddr(ctx context.Context) (string, error) {
	mods, err := m.mc.Discovery.FindByCapability(ctx, capMediaAutomation)
	if err != nil {
		return "", err
	}
	if len(mods) == 0 {
		return "", fmt.Errorf("no automation module")
	}
	addr := mods[0].GetHttpAddr()
	if addr == "" {
		return "", fmt.Errorf("automation module has no address")
	}
	return addr, nil
}

func releaseMatchesToIndexerResults(matches []*automationv1.ReleaseMatch) []*mediaadminv1.IndexerResult {
	results := make([]*mediaadminv1.IndexerResult, 0, len(matches))
	for _, match := range matches {
		if match == nil {
			continue
		}
		results = append(results, releaseMatchToIndexerResult(match))
	}
	return results
}

func releaseMatchToIndexerResult(match *automationv1.ReleaseMatch) *mediaadminv1.IndexerResult {
	return &mediaadminv1.IndexerResult{
		Title:       match.GetTitle(),
		Guid:        match.GetGuid(),
		Indexer:     match.GetIndexerName(),
		Size:        match.GetSize(),
		Quality:     qualityFromReleaseTitle(match.GetTitle()),
		Seeders:     match.GetSeeders(),
		DownloadUrl: match.GetDownloadUrl(),
		Approved:    true,
	}
}

func qualityFromReleaseTitle(title string) string {
	upper := strings.ToUpper(title)
	for _, q := range []struct {
		needle string
		out    string
	}{
		{"FLAC", "FLAC"},
		{"320", "MP3-320"},
		{"V0", "MP3-V0"},
		{"MP3", "MP3"},
	} {
		if strings.Contains(upper, q.needle) {
			return q.out
		}
	}
	return ""
}
