package internal

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// audio extensions recognized during library scans (local files only; no metadata APIs).
var audioExts = map[string]struct{}{
	".mp3":  {},
	".flac": {},
	".m4a":  {},
	".aac":  {},
	".ogg":  {},
	".opus": {},
	".wav":  {},
	".wma":  {},
}

// ScanResult summarizes a library root scan.
type ScanResult struct {
	FilesFound    int
	FilesImported int
	FilesSkipped  int
}

// ScanLibraryRoot walks root for audio files and upserts artists/albums/tracks.
// Layout expected: Artist/Album/track.ext (Artist/track.ext → "Unknown Album").
// Metadata is derived only from path/filename — no network lookups.
func (s *Store) ScanLibraryRoot(root string) (*ScanResult, error) {
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("library root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("library root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("library root is not a directory: %s", root)
	}

	res := &ScanResult{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if _, ok := audioExts[ext]; !ok {
			return nil
		}
		res.FilesFound++
		abs, err := filepath.Abs(path)
		if err != nil {
			res.FilesSkipped++
			return nil
		}
		artistName, albumTitle, trackTitle := inferFromPath(root, abs)
		imported, err := s.importAudioFile(artistName, albumTitle, trackTitle, abs)
		if err != nil {
			return err
		}
		if imported {
			res.FilesImported++
		} else {
			res.FilesSkipped++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Store) importAudioFile(artistName, albumTitle, trackTitle, absPath string) (imported bool, err error) {
	existing, err := s.findTrackByPath(absPath)
	if err != nil {
		return false, err
	}

	ar, err := s.findArtistByName(artistName)
	if err != nil {
		return false, err
	}
	if ar == nil {
		ar, err = s.AddArtist(Artist{
			Name:      artistName,
			Monitored: true,
			Path:      filepath.Dir(filepath.Dir(absPath)),
		})
		if err != nil {
			return false, err
		}
	}

	al, err := s.findAlbum(ar.ID, albumTitle)
	if err != nil {
		return false, err
	}
	if al == nil {
		al, err = s.AddAlbum(Album{
			ArtistID:  ar.ID,
			Title:     albumTitle,
			Monitored: true,
		})
		if err != nil {
			return false, err
		}
	}

	_, err = s.upsertTrack(Track{
		AlbumID:  al.ID,
		ArtistID: ar.ID,
		Title:    trackTitle,
		Path:     absPath,
	})
	if err != nil {
		return false, err
	}
	return existing == nil, nil
}

// inferFromPath derives artist/album/track from Artist/Album/file.ext under root.
func inferFromPath(root, absPath string) (artist, album, track string) {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		rel = filepath.Base(absPath)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	base := parts[len(parts)-1]
	track = stripTrackNumber(strings.TrimSuffix(base, filepath.Ext(base)))
	switch len(parts) {
	case 1:
		return "Unknown Artist", "Unknown Album", track
	case 2:
		return parts[0], "Unknown Album", track
	default:
		return parts[0], parts[1], track
	}
}

func stripTrackNumber(name string) string {
	name = strings.TrimSpace(name)
	i := 0
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
	}
	if i == 0 {
		return name
	}
	rest := strings.TrimSpace(name[i:])
	if strings.HasPrefix(rest, "-") || strings.HasPrefix(rest, ".") || strings.HasPrefix(rest, "_") {
		rest = strings.TrimSpace(rest[1:])
		if rest != "" {
			return rest
		}
	}
	return name
}
