package internal

import (
	"context"
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mediaadminv1 "github.com/Muxcore-Media/contracts-media-admin/gen/muxcore/media/admin/v1"
	musicv1 "github.com/Muxcore-Media/media-music/proto/gen/muxcore/music/v1"
)

func (m *Module) GetMediaTypeInfo(ctx context.Context, _ *mediaadminv1.GetMediaTypeInfoRequest) (*mediaadminv1.GetMediaTypeInfoResponse, error) {
	return &mediaadminv1.GetMediaTypeInfoResponse{
		DisplayName: "Music",
		Icon:        "🎵",
		FilterFields: []*mediaadminv1.FilterField{
			{Key: "year", Label: "Year", Type: mediaadminv1.FilterFieldType_FILTER_FIELD_TYPE_NUMBER},
			{Key: "has_file", Label: "Has File", Type: mediaadminv1.FilterFieldType_FILTER_FIELD_TYPE_SELECT, Options: []string{"true", "false"}},
		},
		Features: []mediaadminv1.Feature{mediaadminv1.Feature_FEATURE_MISSING, mediaadminv1.Feature_FEATURE_TAGS},
	}, nil
}

func (m *Module) ListItems(ctx context.Context, req *mediaadminv1.ListItemsRequest) (*mediaadminv1.ListItemsResponse, error) {
	if m.store == nil {
		return nil, fmt.Errorf("not initialized")
	}
	page := int(req.GetPage())
	if page < 1 {
		page = 1
	}
	pageSize := int(req.GetPageSize())
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	items, total, err := m.store.ListArtistsPage(ctx, req.GetSearch(), req.GetTagId(), page, pageSize, sortFieldToColumn(req.GetSortBy()), req.GetSortOrder())
	if err != nil {
		return nil, err
	}
	out := make([]*mediaadminv1.MediaItem, 0, len(items))
	for _, a := range items {
		out = append(out, m.artistToMediaItem(a))
	}
	return &mediaadminv1.ListItemsResponse{
		Items: out, Total: int32FromInt(total), Page: int32FromInt(page), PageSize: int32FromInt(pageSize),
	}, nil
}

func (m *Module) GetItem(ctx context.Context, req *mediaadminv1.GetItemRequest) (*mediaadminv1.GetItemResponse, error) {
	a, err := mustArtist(ctx, m.store, req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	return &mediaadminv1.GetItemResponse{Item: m.artistToMediaItem(a)}, nil
}

func (m *Module) UpdateMetadata(ctx context.Context, req *mediaadminv1.UpdateMetadataRequest) (*mediaadminv1.UpdateMetadataResponse, error) {
	fields := map[string]any{
		"name": req.GetTitle(), "overview": req.GetDescription(),
	}
	if req.GetMetadata() != nil {
		if v := req.GetMetadata()["quality_profile_id"]; v != "" {
			fields["quality_profile_id"] = v
		}
		if v := req.GetMetadata()["root_folder_path"]; v != "" {
			fields["root_folder_path"] = v
		}
		if v := req.GetMetadata()["monitored"]; v != "" {
			fields["monitored"] = v == "true" || v == "1"
		}
	}
	a, err := m.store.UpdateArtist(ctx, req.GetId(), fields)
	if err != nil {
		return nil, err
	}
	return &mediaadminv1.UpdateMetadataResponse{Item: m.artistToMediaItem(a)}, nil
}

func (m *Module) ListArtwork(ctx context.Context, req *mediaadminv1.ListArtworkRequest) (*mediaadminv1.ListArtworkResponse, error) {
	a, err := mustArtist(ctx, m.store, req.GetId())
	if err != nil {
		return nil, err
	}
	var artwork []*mediaadminv1.ArtworkInfo
	if url := m.servableArtworkURL(a.PosterURL); url != "" {
		artwork = append(artwork, &mediaadminv1.ArtworkInfo{
			Id: a.ID + "_poster", ItemId: a.ID,
			Type: mediaadminv1.ArtworkType_ARTWORK_TYPE_POSTER, Url: url,
		})
	}
	return &mediaadminv1.ListArtworkResponse{Artwork: artwork}, nil
}

func (m *Module) DeleteItem(ctx context.Context, req *mediaadminv1.DeleteItemRequest) (*mediaadminv1.DeleteItemResponse, error) {
	if err := m.store.RemoveArtist(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &mediaadminv1.DeleteItemResponse{}, nil
}

func (m *Module) RefreshItem(ctx context.Context, req *mediaadminv1.RefreshItemRequest) (*mediaadminv1.RefreshItemResponse, error) {
	srv := &musicServer{m: m}
	if _, err := srv.RefreshMetadata(ctx, &musicv1.RefreshMetadataRequest{ArtistId: req.GetId()}); err != nil {
		return nil, err
	}
	item, err := m.GetItem(ctx, &mediaadminv1.GetItemRequest{Id: req.GetId()})
	if err != nil {
		return nil, err
	}
	return &mediaadminv1.RefreshItemResponse{Item: item.GetItem()}, nil
}

func (m *Module) artistToMediaItem(a *Artist) *mediaadminv1.MediaItem {
	meta := map[string]string{
		"musicbrainz_id":     a.MusicBrainzID,
		"monitored":          strconv.FormatBool(a.Monitored),
		"has_file":           strconv.FormatBool(a.HasFile),
		"quality_profile_id": a.QualityProfileID,
		"root_folder_path":   a.RootFolderPath,
		"path":               a.Path,
	}
	var artwork []*mediaadminv1.ArtworkInfo
	if url := m.servableArtworkURL(a.PosterURL); url != "" {
		artwork = append(artwork, &mediaadminv1.ArtworkInfo{Type: mediaadminv1.ArtworkType_ARTWORK_TYPE_POSTER, Url: url})
	}
	return &mediaadminv1.MediaItem{
		Id: a.ID, Title: a.Name, Description: a.Overview,
		Metadata: meta, Artwork: artwork, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}
