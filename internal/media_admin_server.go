package internal

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mediaadminv1 "github.com/Muxcore-Media/contracts-media-admin/gen/muxcore/media/admin/v1"
	musicv1 "github.com/Muxcore-Media/media-music/proto/gen/muxcore/music/v1"
)

type mediaAdminServer struct {
	mediaadminv1.UnimplementedMediaAdminServiceServer
	m *Module
}

func (s mediaAdminServer) GetMediaTypeInfo(ctx context.Context, req *mediaadminv1.GetMediaTypeInfoRequest) (*mediaadminv1.GetMediaTypeInfoResponse, error) {
	return s.m.GetMediaTypeInfo(ctx, req)
}

func (s mediaAdminServer) ListItems(ctx context.Context, req *mediaadminv1.ListItemsRequest) (*mediaadminv1.ListItemsResponse, error) {
	return s.m.ListItems(ctx, req)
}

func (s mediaAdminServer) GetItem(ctx context.Context, req *mediaadminv1.GetItemRequest) (*mediaadminv1.GetItemResponse, error) {
	return s.m.GetItem(ctx, req)
}

func (s mediaAdminServer) UpdateMetadata(ctx context.Context, req *mediaadminv1.UpdateMetadataRequest) (*mediaadminv1.UpdateMetadataResponse, error) {
	return s.m.UpdateMetadata(ctx, req)
}

func (s mediaAdminServer) ListArtwork(ctx context.Context, req *mediaadminv1.ListArtworkRequest) (*mediaadminv1.ListArtworkResponse, error) {
	return s.m.ListArtwork(ctx, req)
}

func (s mediaAdminServer) ReplaceArtwork(stream mediaadminv1.MediaAdminService_ReplaceArtworkServer) error {
	return s.m.ReplaceArtwork(stream)
}

func (s mediaAdminServer) DeleteItem(ctx context.Context, req *mediaadminv1.DeleteItemRequest) (*mediaadminv1.DeleteItemResponse, error) {
	return s.m.DeleteItem(ctx, req)
}

func (s mediaAdminServer) RefreshItem(ctx context.Context, req *mediaadminv1.RefreshItemRequest) (*mediaadminv1.RefreshItemResponse, error) {
	return s.m.RefreshItem(ctx, req)
}

func (s mediaAdminServer) SearchIndexers(ctx context.Context, req *mediaadminv1.SearchIndexersRequest) (*mediaadminv1.SearchIndexersResponse, error) {
	return s.m.SearchIndexers(ctx, req)
}

func (s mediaAdminServer) ListHistory(ctx context.Context, req *mediaadminv1.ListHistoryRequest) (*mediaadminv1.ListHistoryResponse, error) {
	return s.m.ListHistory(ctx, req)
}

func (s mediaAdminServer) ListMissing(ctx context.Context, req *mediaadminv1.ListMissingRequest) (*mediaadminv1.ListMissingResponse, error) {
	resp, err := (&musicServer{m: s.m}).ListMissing(ctx, &musicv1.ListMissingRequest{
		Page: req.GetPage(), PageSize: req.GetPageSize(), ArtistId: req.GetParentId(),
	})
	if err != nil {
		return nil, err
	}
	items := make([]*mediaadminv1.MissingItem, 0, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		items = append(items, &mediaadminv1.MissingItem{
			Id: it.GetAlbumId(), ParentId: it.GetArtistId(),
			Title: it.GetTitle(), Year: it.GetYear(),
			Metadata: map[string]string{
				"artist_name":        it.GetArtistName(),
				"musicbrainz_id":     it.GetMusicbrainzId(),
				"quality_profile_id": it.GetQualityProfileId(),
				"root_folder_path":   it.GetRootFolderPath(),
			},
		})
	}
	return &mediaadminv1.ListMissingResponse{
		Items: items, Total: resp.GetTotal(), Page: resp.GetPage(), PageSize: resp.GetPageSize(),
	}, nil
}

func (s mediaAdminServer) ListTags(ctx context.Context, req *mediaadminv1.ListTagsRequest) (*mediaadminv1.ListTagsResponse, error) {
	resp, err := (&musicServer{m: s.m}).ListTags(ctx, &musicv1.ListTagsRequest{})
	if err != nil {
		return nil, err
	}
	tags := make([]*mediaadminv1.Tag, 0, len(resp.GetTags()))
	for _, t := range resp.GetTags() {
		tags = append(tags, &mediaadminv1.Tag{Id: t.GetId(), Label: t.GetLabel(), CreatedAt: t.GetCreatedAt()})
	}
	return &mediaadminv1.ListTagsResponse{Tags: tags}, nil
}

func (s mediaAdminServer) CreateTag(ctx context.Context, req *mediaadminv1.CreateTagRequest) (*mediaadminv1.CreateTagResponse, error) {
	resp, err := (&musicServer{m: s.m}).CreateTag(ctx, &musicv1.CreateTagRequest{Label: req.GetLabel()})
	if err != nil {
		return nil, err
	}
	return &mediaadminv1.CreateTagResponse{TagId: resp.GetTagId()}, nil
}

func (s mediaAdminServer) DeleteTag(ctx context.Context, req *mediaadminv1.DeleteTagRequest) (*mediaadminv1.DeleteTagResponse, error) {
	if _, err := (&musicServer{m: s.m}).DeleteTag(ctx, &musicv1.DeleteTagRequest{TagId: req.GetTagId()}); err != nil {
		return nil, err
	}
	return &mediaadminv1.DeleteTagResponse{}, nil
}

func (s mediaAdminServer) SetItemTags(ctx context.Context, req *mediaadminv1.SetItemTagsRequest) (*mediaadminv1.SetItemTagsResponse, error) {
	if _, err := (&musicServer{m: s.m}).SetItemTags(ctx, &musicv1.SetItemTagsRequest{
		ItemId: req.GetItemId(), TagIds: req.GetTagIds(),
	}); err != nil {
		return nil, err
	}
	return &mediaadminv1.SetItemTagsResponse{}, nil
}

func (s mediaAdminServer) ListCollections(_ context.Context, _ *mediaadminv1.ListCollectionsRequest) (*mediaadminv1.ListCollectionsResponse, error) {
	return &mediaadminv1.ListCollectionsResponse{}, nil
}

func (s mediaAdminServer) GetCollectionItems(_ context.Context, _ *mediaadminv1.GetCollectionItemsRequest) (*mediaadminv1.GetCollectionItemsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "collections are not supported for music")
}

func (s mediaAdminServer) GetCalendar(_ context.Context, _ *mediaadminv1.GetCalendarRequest) (*mediaadminv1.GetCalendarResponse, error) {
	return nil, status.Error(codes.Unimplemented, "calendar is not supported for music")
}
