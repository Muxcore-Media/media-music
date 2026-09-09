package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	musicv1 "github.com/Muxcore-Media/media-music/proto/gen/muxcore/music/v1"
)

type musicServer struct {
	musicv1.UnimplementedMusicManagementServiceServer
	m *Module
}

func (s *musicServer) AddArtist(ctx context.Context, req *musicv1.AddArtistRequest) (*musicv1.AddArtistResponse, error) {
	a, err := s.m.store.AddArtist(ctx, Artist{
		Name: req.GetName(), MusicBrainzID: req.GetMusicbrainzId(),
		Monitored: req.GetMonitored(), Path: req.GetPath(),
		QualityProfileID: req.GetQualityProfileId(), RootFolderPath: req.GetRootFolderPath(),
	})
	if err != nil {
		return nil, err
	}
	if syncErr := s.m.syncArtistMetadataFromMusicBrainz(ctx, a); syncErr != nil {
		slog.Warn("music metadata sync on add artist", "artist", a.Name, "error", syncErr)
	}
	if updated, getErr := s.m.store.GetArtist(ctx, a.ID); getErr == nil {
		a = updated
	}
	s.m.store.appendHistory(ctx, a.ID, historyImport, a.Name, "")
	return &musicv1.AddArtistResponse{Artist: toPBArtist(a)}, nil
}

func (s *musicServer) RemoveArtist(ctx context.Context, req *musicv1.RemoveArtistRequest) (*musicv1.RemoveArtistResponse, error) {
	if req.GetDeleteFiles() {
		tracks, _ := s.m.store.ListTracks(ctx, "", req.GetId())
		for _, t := range tracks {
			if err := s.m.requirePathAllowed(ctx, t.Path, t.ArtistID); err != nil {
				continue
			}
			_ = os.Remove(t.Path)
		}
	}
	if err := s.m.store.RemoveArtist(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &musicv1.RemoveArtistResponse{}, nil
}

func (s *musicServer) RefreshMetadata(ctx context.Context, req *musicv1.RefreshMetadataRequest) (*musicv1.RefreshMetadataResponse, error) {
	a, err := s.m.store.GetArtist(ctx, req.GetArtistId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}
	if err := s.m.syncArtistMetadataFromMusicBrainz(ctx, a); err != nil {
		slog.Warn("music metadata refresh", "artist", a.Name, "error", err)
	}
	root := a.Path
	if root == "" {
		root = s.m.getLibraryDir()
	}
	if _, err := s.m.store.ScanLibraryRoot(ctx, root); err != nil {
		return nil, err
	}
	return &musicv1.RefreshMetadataResponse{}, nil
}

func (s *musicServer) ListArtists(ctx context.Context, req *musicv1.ListArtistsRequest) (*musicv1.ListArtistsResponse, error) {
	items, total, err := s.m.store.ListArtistsPage(ctx, req.GetQuery(), req.GetTagId(), int(req.GetPage()), int(req.GetPageSize()), req.GetSortBy(), req.GetSortOrder())
	if err != nil {
		return nil, err
	}
	page := int(req.GetPage())
	if page < 1 {
		page = 1
	}
	pageSize := int(req.GetPageSize())
	if pageSize < 1 {
		pageSize = 50
	}
	out := make([]*musicv1.Artist, 0, len(items))
	for _, a := range items {
		out = append(out, toPBArtist(a))
	}
	return &musicv1.ListArtistsResponse{Artists: out, Total: int32FromInt(total), Page: int32FromInt(page), PageSize: int32FromInt(pageSize)}, nil
}

func (s *musicServer) ListMissing(ctx context.Context, req *musicv1.ListMissingRequest) (*musicv1.ListMissingResponse, error) {
	items, total, err := s.m.store.ListMissing(ctx, int(req.GetPage()), int(req.GetPageSize()), req.GetArtistId())
	if err != nil {
		return nil, err
	}
	page := int(req.GetPage())
	if page < 1 {
		page = 1
	}
	pageSize := int(req.GetPageSize())
	if pageSize < 1 {
		pageSize = 50
	}
	out := make([]*musicv1.MissingAlbumItem, 0, len(items))
	for _, it := range items {
		out = append(out, &musicv1.MissingAlbumItem{
			AlbumId: it.AlbumID, ArtistId: it.ArtistID, ArtistName: it.ArtistName,
			Title: it.Title, Year: it.Year, MusicbrainzId: it.MusicBrainzID,
			QualityProfileId: it.QualityProfileID, RootFolderPath: it.RootFolderPath,
		})
	}
	return &musicv1.ListMissingResponse{Items: out, Total: int32FromInt(total), Page: int32FromInt(page), PageSize: int32FromInt(pageSize)}, nil
}

func (s *musicServer) GetArtist(ctx context.Context, req *musicv1.GetArtistRequest) (*musicv1.GetArtistResponse, error) {
	a, err := s.m.store.GetArtist(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &musicv1.GetArtistResponse{Artist: toPBArtist(a)}, nil
}

func (s *musicServer) UpdateArtist(ctx context.Context, req *musicv1.UpdateArtistRequest) (*musicv1.UpdateArtistResponse, error) {
	fields := map[string]any{}
	if req.QualityProfileId != nil {
		fields["quality_profile_id"] = req.GetQualityProfileId()
	}
	if req.RootFolderPath != nil {
		fields["root_folder_path"] = req.GetRootFolderPath()
	}
	if req.Monitored != nil {
		fields["monitored"] = req.GetMonitored()
	}
	if req.Name != nil {
		fields["name"] = req.GetName()
	}
	if req.Overview != nil {
		fields["overview"] = req.GetOverview()
	}
	a, err := s.m.store.UpdateArtist(ctx, req.GetId(), fields)
	if err != nil {
		return nil, err
	}
	return &musicv1.UpdateArtistResponse{Artist: toPBArtist(a)}, nil
}

func (s *musicServer) AddAlbum(ctx context.Context, req *musicv1.AddAlbumRequest) (*musicv1.AddAlbumResponse, error) {
	al, err := s.m.store.AddAlbum(ctx, Album{
		ArtistID: req.GetArtistId(), Title: req.GetTitle(),
		MusicBrainzID: req.GetMusicbrainzId(), Year: req.GetYear(), Monitored: req.GetMonitored(),
	})
	if err != nil {
		return nil, err
	}
	return &musicv1.AddAlbumResponse{Album: toPBAlbum(al)}, nil
}

func (s *musicServer) ListAlbums(ctx context.Context, req *musicv1.ListAlbumsRequest) (*musicv1.ListAlbumsResponse, error) {
	items, err := s.m.store.ListAlbums(ctx, req.GetArtistId())
	if err != nil {
		return nil, err
	}
	out := make([]*musicv1.Album, 0, len(items))
	for _, a := range items {
		out = append(out, toPBAlbum(a))
	}
	return &musicv1.ListAlbumsResponse{Albums: out}, nil
}

func (s *musicServer) UpdateAlbumMonitored(ctx context.Context, req *musicv1.UpdateAlbumMonitoredRequest) (*musicv1.UpdateAlbumMonitoredResponse, error) {
	if err := s.m.store.UpdateAlbumMonitored(ctx, req.GetAlbumId(), req.GetMonitored()); err != nil {
		return nil, err
	}
	return &musicv1.UpdateAlbumMonitoredResponse{}, nil
}

func (s *musicServer) AddTrackFile(ctx context.Context, req *musicv1.AddTrackFileRequest) (*musicv1.AddTrackFileResponse, error) {
	al, err := s.m.store.findAlbumByID(ctx, req.GetAlbumId())
	if err != nil {
		return nil, err
	}
	if pathErr := s.m.requirePathAllowed(ctx, req.GetFilePath(), al.ArtistID); pathErr != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", pathErr)
	}
	t, err := s.m.store.AddTrackFile(ctx, req.GetAlbumId(), req.GetFilePath(), req.GetTitle(), req.GetQuality(), req.GetContainer(), req.GetSizeBytes())
	if err != nil {
		return nil, err
	}
	s.m.store.appendHistory(ctx, t.ArtistID, historyImport, t.Title, t.Quality)
	return &musicv1.AddTrackFileResponse{FileId: t.ID}, nil
}

func (s *musicServer) RemoveTrackFile(ctx context.Context, req *musicv1.RemoveTrackFileRequest) (*musicv1.RemoveTrackFileResponse, error) {
	t, err := s.m.store.GetTrack(ctx, req.GetFileId())
	if err != nil {
		return nil, err
	}
	if req.GetDeleteFiles() {
		if err := s.m.requirePathAllowed(ctx, t.Path, t.ArtistID); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
	}
	if err := s.m.store.RemoveTrack(ctx, req.GetFileId(), req.GetDeleteFiles()); err != nil {
		return nil, err
	}
	s.m.store.appendHistory(ctx, t.ArtistID, historyDeleteFile, t.Title, t.Quality)
	return &musicv1.RemoveTrackFileResponse{}, nil
}

func (s *musicServer) ListTrackFiles(ctx context.Context, req *musicv1.ListTrackFilesRequest) (*musicv1.ListTrackFilesResponse, error) {
	tracks, err := s.m.store.ListTracks(ctx, req.GetAlbumId(), req.GetArtistId())
	if err != nil {
		return nil, err
	}
	out := make([]*musicv1.TrackFile, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, toPBTrackFile(t))
	}
	return &musicv1.ListTrackFilesResponse{Files: out}, nil
}

func (s *musicServer) CreateTag(ctx context.Context, req *musicv1.CreateTagRequest) (*musicv1.CreateTagResponse, error) {
	id, err := s.m.store.CreateTag(ctx, req.GetLabel())
	if err != nil {
		return nil, err
	}
	return &musicv1.CreateTagResponse{TagId: id}, nil
}

func (s *musicServer) DeleteTag(ctx context.Context, req *musicv1.DeleteTagRequest) (*musicv1.DeleteTagResponse, error) {
	if err := s.m.store.DeleteTag(ctx, req.GetTagId()); err != nil {
		return nil, err
	}
	return &musicv1.DeleteTagResponse{}, nil
}

func (s *musicServer) ListTags(ctx context.Context, _ *musicv1.ListTagsRequest) (*musicv1.ListTagsResponse, error) {
	tags, err := s.m.store.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*musicv1.Tag, 0, len(tags))
	for _, t := range tags {
		out = append(out, &musicv1.Tag{Id: t.ID, Label: t.Label, CreatedAt: t.CreatedAt})
	}
	return &musicv1.ListTagsResponse{Tags: out}, nil
}

func (s *musicServer) SetItemTags(ctx context.Context, req *musicv1.SetItemTagsRequest) (*musicv1.SetItemTagsResponse, error) {
	if err := s.m.store.SetItemTags(ctx, req.GetItemId(), req.GetTagIds()); err != nil {
		return nil, err
	}
	return &musicv1.SetItemTagsResponse{}, nil
}

func (s *musicServer) GetItemTags(ctx context.Context, req *musicv1.GetItemTagsRequest) (*musicv1.GetItemTagsResponse, error) {
	tags, err := s.m.store.GetItemTags(ctx, req.GetItemId())
	if err != nil {
		return nil, err
	}
	out := make([]*musicv1.Tag, 0, len(tags))
	for _, t := range tags {
		out = append(out, &musicv1.Tag{Id: t.ID, Label: t.Label, CreatedAt: t.CreatedAt})
	}
	return &musicv1.GetItemTagsResponse{Tags: out}, nil
}

func toPBArtist(a *Artist) *musicv1.Artist {
	return &musicv1.Artist{
		Id: a.ID, Name: a.Name, MusicbrainzId: a.MusicBrainzID,
		Monitored: a.Monitored, Path: a.Path, HasFile: a.HasFile,
		QualityProfileId: a.QualityProfileID, RootFolderPath: a.RootFolderPath,
		Overview: a.Overview, PosterUrl: a.PosterURL,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func toPBAlbum(a *Album) *musicv1.Album {
	return &musicv1.Album{
		Id: a.ID, ArtistId: a.ArtistID, Title: a.Title,
		MusicbrainzId: a.MusicBrainzID, Year: a.Year, Monitored: a.Monitored, HasFile: a.HasFile,
	}
}

func toPBTrackFile(t *Track) *musicv1.TrackFile {
	return &musicv1.TrackFile{
		Id: t.ID, ArtistId: t.ArtistID, AlbumId: t.AlbumID, Title: t.Title,
		FilePath: t.Path, Quality: t.Quality, SizeBytes: t.SizeBytes,
		Container: t.Container, CreatedAt: t.CreatedAt,
	}
}

func trackContainer(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext == "" {
		return "unknown"
	}
	return ext
}

func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.Size()
}

func mustArtist(ctx context.Context, store *Store, id string) (*Artist, error) {
	if store == nil {
		return nil, fmt.Errorf("store not open")
	}
	return store.GetArtist(ctx, id)
}
