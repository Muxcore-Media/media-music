package internal

import (
	"context"
	"log/slog"
	"strings"

	musicbrainzv1 "github.com/Muxcore-Media/metadata-musicbrainz/proto/musicbrainzv1"
)

func (m *Module) syncArtistMetadataFromMusicBrainz(ctx context.Context, a *Artist) error {
	if a == nil {
		return nil
	}
	mbid := strings.TrimSpace(a.MusicBrainzID)
	if mbid == "" {
		return nil
	}
	cli, closeFn, err := m.musicBrainzClient(ctx)
	if err != nil {
		return err
	}
	defer closeFn()

	resp, err := cli.GetArtist(ctx, &musicbrainzv1.GetArtistRequest{MusicbrainzId: mbid})
	if err != nil {
		return err
	}
	overview := strings.TrimSpace(resp.GetDisambiguation())
	if overview != "" && overview != a.Overview {
		_, _ = m.store.UpdateArtist(a.ID, map[string]any{"overview": overview})
	}
	for _, rg := range resp.GetReleaseGroups() {
		if rg == nil || strings.TrimSpace(rg.GetTitle()) == "" {
			continue
		}
		if pt := strings.ToLower(strings.TrimSpace(rg.GetPrimaryType())); pt != "" && pt != "album" && pt != "ep" && pt != "single" {
			continue
		}
		_, err := m.store.EnsureAlbum(a.ID, Album{
			Title:         rg.GetTitle(),
			MusicBrainzID: rg.GetId(),
			Year:          rg.GetFirstReleaseDateYear(),
			Monitored:     true,
		})
		if err != nil {
			slog.Debug("ensure album from musicbrainz", "artist", a.Name, "album", rg.GetTitle(), "error", err)
		}
	}
	slog.Info("music metadata synced", "artist", a.Name, "mbid", mbid, "release_groups", len(resp.GetReleaseGroups()))
	return nil
}

func (m *Module) searchArtistMetadata(ctx context.Context, query string, limit int32) ([]*musicbrainzv1.ArtistSearchResult, error) {
	cli, closeFn, err := m.musicBrainzClient(ctx)
	if err != nil {
		return nil, err
	}
	defer closeFn()
	resp, err := cli.SearchArtist(ctx, &musicbrainzv1.SearchArtistRequest{Query: query, Limit: limit})
	if err != nil {
		return nil, err
	}
	return resp.GetArtists(), nil
}
