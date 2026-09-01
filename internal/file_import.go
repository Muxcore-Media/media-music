package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Muxcore-Media/core/pkg/contracts"
)

const fileImportRetryInterval = 2 * time.Second

func (m *Module) subscribeToFileImported(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		m.mu.RLock()
		mc := m.mc
		m.mu.RUnlock()
		if mc != nil {
			break
		}
		time.Sleep(fileImportRetryInterval)
	}
	m.mu.RLock()
	mc := m.mc
	m.mu.RUnlock()
	if mc == nil {
		return
	}
	ch, cancel, err := mc.Events.Subscribe(ctx, contracts.EventFileImported)
	if err != nil {
		slog.Warn("media-music: subscribe file imported", "error", err)
		return
	}
	go func() {
		for evt := range ch {
			var p contracts.FileImportedPayload
			if err := json.Unmarshal(evt.Payload, &p); err != nil {
				continue
			}
			if !isMusicMediaType(p.MediaType) {
				continue
			}
			if err := m.handleMusicFileImported(ctx, p); err != nil {
				slog.Debug("media-music: handle imported file", "title", p.Title, "error", err)
			}
		}
		cancel()
	}()
	slog.Info("media-music: subscribed to file imported events")
}

func isMusicMediaType(t string) bool {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "music", "audio", "track", "album":
		return true
	default:
		return false
	}
}

func (m *Module) handleMusicFileImported(ctx context.Context, p contracts.FileImportedPayload) error {
	if m.store == nil {
		return errNotInit
	}
	path := p.DestinationPath
	if path == "" {
		path = p.StorageKey
	}
	if path == "" {
		path = p.OriginalPath
	}
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.getLibraryDir(), path)
	}
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return err
	}
	artistName, albumTitle, trackTitle := inferFromPath(m.getLibraryDir(), path)
	if p.Title != "" {
		trackTitle = p.Title
	}
	ar, err := m.store.findArtistByName(ctx, artistName)
	if err != nil {
		return err
	}
	if ar == nil {
		ar, err = m.store.AddArtist(ctx, Artist{Name: artistName, Monitored: true, Path: filepath.Dir(filepath.Dir(path))})
		if err != nil {
			return err
		}
	}
	al, err := m.store.findAlbum(ctx, ar.ID, albumTitle)
	if err != nil {
		return err
	}
	if al == nil {
		al, err = m.store.AddAlbum(ctx, Album{ArtistID: ar.ID, Title: albumTitle, Monitored: true})
		if err != nil {
			return err
		}
	}
	quality := p.Quality
	if quality == "" {
		quality = "unknown"
	}
	_, err = m.store.AddTrackFile(ctx, al.ID, path, trackTitle, quality, trackContainer(path), st.Size())
	if err != nil {
		return err
	}
	m.store.appendHistory(ctx, ar.ID, historyImport, trackTitle, quality)
	return nil
}
