package internal

import (
	"context"

	automationv1 "github.com/Muxcore-Media/contracts-automation/muxcore/automation/v1"
	mediaadminv1 "github.com/Muxcore-Media/contracts-media-admin/gen/muxcore/media/admin/v1"
	"github.com/Muxcore-Media/core/pkg/contracts"
)

// SetMusicBrainzOverride wires a test double for metadata-musicbrainz RPCs.
func (m *Module) SetMusicBrainzOverride(api musicBrainzAPI) {
	m.musicBrainzOverride = api
}

// SetAutomationSearchOverride wires automation SearchItem for tests.
func (m *Module) SetAutomationSearchOverride(fn func(context.Context, *automationv1.SearchItemRequest) (*automationv1.SearchItemResponse, error)) {
	m.automationSearchFn = fn
}

// Store exposes the SQLite store for tests.
func (m *Module) Store() *Store {
	return m.store
}

// SyncArtistMetadata runs MusicBrainz sync for tests.
func (m *Module) SyncArtistMetadata(ctx context.Context, a *Artist) error {
	return m.syncArtistMetadataFromMusicBrainz(ctx, a)
}

// HandleMusicFileImported upserts artist/album/track from an import event (tests).
func (m *Module) HandleMusicFileImported(ctx context.Context, p contracts.FileImportedPayload) error {
	return m.handleMusicFileImported(ctx, p)
}

// ListMissingAdmin exposes MediaAdmin ListMissing for tests.
func (m *Module) ListMissingAdmin(ctx context.Context, req *mediaadminv1.ListMissingRequest) (*mediaadminv1.ListMissingResponse, error) {
	return mediaAdminServer{m: m}.ListMissing(ctx, req)
}

// ListTagsAdmin exposes MediaAdmin ListTags for tests.
func (m *Module) ListTagsAdmin(ctx context.Context, req *mediaadminv1.ListTagsRequest) (*mediaadminv1.ListTagsResponse, error) {
	return mediaAdminServer{m: m}.ListTags(ctx, req)
}

// ListHistoryAdmin exposes MediaAdmin ListHistory for tests.
func (m *Module) ListHistoryAdmin(ctx context.Context, req *mediaadminv1.ListHistoryRequest) (*mediaadminv1.ListHistoryResponse, error) {
	return m.ListHistory(ctx, req)
}

// AppendHistory records a history row for tests.
func (s *Store) AppendHistory(ctx context.Context, itemID, eventType, sourceTitle, quality string) {
	s.appendHistory(ctx, itemID, eventType, sourceTitle, quality)
}
