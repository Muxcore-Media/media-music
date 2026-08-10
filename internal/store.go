package internal

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Artist struct {
	ID            string
	Name          string
	MusicBrainzID string
	Monitored     bool
	Path          string
}

type Album struct {
	ID            string
	ArtistID      string
	Title         string
	MusicBrainzID string
	Year          int32
	Monitored     bool
}

// Store is an in-memory library (v0.1.0 scaffold).
type Store struct {
	mu      sync.RWMutex
	artists map[string]*Artist
	albums  map[string]*Album
}

func NewStore() *Store {
	return &Store{
		artists: map[string]*Artist{},
		albums:  map[string]*Album{},
	}
}

func (s *Store) AddArtist(a Artist) (*Artist, error) {
	if strings.TrimSpace(a.Name) == "" {
		return nil, fmt.Errorf("artist name required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ID == "" {
		a.ID = "ar_" + uuid.NewString()[:8]
	}
	cp := a
	s.artists[cp.ID] = &cp
	out := cp
	return &out, nil
}

func (s *Store) GetArtist(id string) (*Artist, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.artists[id]
	if !ok {
		return nil, fmt.Errorf("artist %q not found", id)
	}
	cp := *a
	return &cp, nil
}

func (s *Store) ListArtists(query string) []*Artist {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]*Artist, 0, len(s.artists))
	for _, a := range s.artists {
		if q != "" && !strings.Contains(strings.ToLower(a.Name), q) {
			continue
		}
		cp := *a
		out = append(out, &cp)
	}
	return out
}

func (s *Store) RemoveArtist(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.artists[id]; !ok {
		return fmt.Errorf("artist %q not found", id)
	}
	delete(s.artists, id)
	for aid, al := range s.albums {
		if al.ArtistID == id {
			delete(s.albums, aid)
		}
	}
	return nil
}

func (s *Store) AddAlbum(al Album) (*Album, error) {
	if strings.TrimSpace(al.Title) == "" {
		return nil, fmt.Errorf("album title required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.artists[al.ArtistID]; !ok {
		return nil, fmt.Errorf("artist %q not found", al.ArtistID)
	}
	if al.ID == "" {
		al.ID = "al_" + uuid.NewString()[:8]
	}
	cp := al
	s.albums[cp.ID] = &cp
	out := cp
	return &out, nil
}

func (s *Store) ListAlbums(artistID string) []*Album {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Album, 0)
	for _, al := range s.albums {
		if artistID != "" && al.ArtistID != artistID {
			continue
		}
		cp := *al
		out = append(out, &cp)
	}
	return out
}
