package internal

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
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

// Track is a single audio file in the library.
type Track struct {
	ID       string
	AlbumID  string
	ArtistID string
	Title    string
	Path     string
}

// Store persists the music library in SQLite.
type Store struct {
	db *sql.DB
}

// OpenStore opens or creates the SQLite database at path (WAL mode).
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS artists (
			id              TEXT PRIMARY KEY,
			name            TEXT NOT NULL,
			musicbrainz_id  TEXT NOT NULL DEFAULT '',
			monitored       INTEGER NOT NULL DEFAULT 1,
			path            TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS albums (
			id              TEXT PRIMARY KEY,
			artist_id       TEXT NOT NULL,
			title           TEXT NOT NULL,
			musicbrainz_id  TEXT NOT NULL DEFAULT '',
			year            INTEGER NOT NULL DEFAULT 0,
			monitored       INTEGER NOT NULL DEFAULT 1,
			FOREIGN KEY (artist_id) REFERENCES artists(id) ON DELETE CASCADE
		);
		CREATE TABLE IF NOT EXISTS tracks (
			id         TEXT PRIMARY KEY,
			album_id   TEXT NOT NULL,
			artist_id  TEXT NOT NULL,
			title      TEXT NOT NULL,
			path       TEXT NOT NULL UNIQUE,
			FOREIGN KEY (album_id) REFERENCES albums(id) ON DELETE CASCADE,
			FOREIGN KEY (artist_id) REFERENCES artists(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_artists_name ON artists(name);
		CREATE INDEX IF NOT EXISTS idx_albums_artist ON albums(artist_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album_id);
		CREATE INDEX IF NOT EXISTS idx_tracks_path ON tracks(path);
	`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) AddArtist(a Artist) (*Artist, error) {
	if strings.TrimSpace(a.Name) == "" {
		return nil, fmt.Errorf("artist name required")
	}
	if a.ID == "" {
		a.ID = "ar_" + uuid.NewString()[:8]
	}
	monitored := 0
	if a.Monitored {
		monitored = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO artists (id, name, musicbrainz_id, monitored, path)
		VALUES (?, ?, ?, ?, ?)
	`, a.ID, a.Name, a.MusicBrainzID, monitored, a.Path)
	if err != nil {
		return nil, fmt.Errorf("insert artist: %w", err)
	}
	out := a
	return &out, nil
}

func (s *Store) GetArtist(id string) (*Artist, error) {
	row := s.db.QueryRow(`
		SELECT id, name, musicbrainz_id, monitored, path FROM artists WHERE id = ?
	`, id)
	return scanArtist(row)
}

func (s *Store) ListArtists(query string) ([]*Artist, error) {
	rows, err := s.db.Query(`
		SELECT id, name, musicbrainz_id, monitored, path FROM artists ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list artists: %w", err)
	}
	defer rows.Close()
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]*Artist, 0)
	for rows.Next() {
		a, err := scanArtist(rows)
		if err != nil {
			return nil, err
		}
		if q != "" && !strings.Contains(strings.ToLower(a.Name), q) {
			continue
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) RemoveArtist(id string) error {
	res, err := s.db.Exec(`DELETE FROM artists WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete artist: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("artist %q not found", id)
	}
	// Cascades may be off without PRAGMA foreign_keys; clean children explicitly.
	_, _ = s.db.Exec(`DELETE FROM tracks WHERE artist_id = ?`, id)
	_, _ = s.db.Exec(`DELETE FROM albums WHERE artist_id = ?`, id)
	return nil
}

func (s *Store) AddAlbum(al Album) (*Album, error) {
	if strings.TrimSpace(al.Title) == "" {
		return nil, fmt.Errorf("album title required")
	}
	var exists string
	err := s.db.QueryRow(`SELECT id FROM artists WHERE id = ?`, al.ArtistID).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("artist %q not found", al.ArtistID)
	}
	if err != nil {
		return nil, fmt.Errorf("lookup artist: %w", err)
	}
	if al.ID == "" {
		al.ID = "al_" + uuid.NewString()[:8]
	}
	monitored := 0
	if al.Monitored {
		monitored = 1
	}
	_, err = s.db.Exec(`
		INSERT INTO albums (id, artist_id, title, musicbrainz_id, year, monitored)
		VALUES (?, ?, ?, ?, ?, ?)
	`, al.ID, al.ArtistID, al.Title, al.MusicBrainzID, al.Year, monitored)
	if err != nil {
		return nil, fmt.Errorf("insert album: %w", err)
	}
	out := al
	return &out, nil
}

func (s *Store) ListAlbums(artistID string) ([]*Album, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if artistID != "" {
		rows, err = s.db.Query(`
			SELECT id, artist_id, title, musicbrainz_id, year, monitored
			FROM albums WHERE artist_id = ? ORDER BY year, title
		`, artistID)
	} else {
		rows, err = s.db.Query(`
			SELECT id, artist_id, title, musicbrainz_id, year, monitored
			FROM albums ORDER BY title
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("list albums: %w", err)
	}
	defer rows.Close()
	out := make([]*Album, 0)
	for rows.Next() {
		al, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, al)
	}
	return out, rows.Err()
}

// ListTracks returns tracks, optionally filtered by album ID.
func (s *Store) ListTracks(albumID string) ([]*Track, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if albumID != "" {
		rows, err = s.db.Query(`
			SELECT id, album_id, artist_id, title, path FROM tracks
			WHERE album_id = ? ORDER BY title
		`, albumID)
	} else {
		rows, err = s.db.Query(`
			SELECT id, album_id, artist_id, title, path FROM tracks ORDER BY title
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("list tracks: %w", err)
	}
	defer rows.Close()
	out := make([]*Track, 0)
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) findArtistByName(name string) (*Artist, error) {
	row := s.db.QueryRow(`
		SELECT id, name, musicbrainz_id, monitored, path FROM artists
		WHERE lower(name) = lower(?) LIMIT 1
	`, name)
	a, err := scanArtist(row)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	return a, nil
}

func (s *Store) findAlbum(artistID, title string) (*Album, error) {
	row := s.db.QueryRow(`
		SELECT id, artist_id, title, musicbrainz_id, year, monitored FROM albums
		WHERE artist_id = ? AND lower(title) = lower(?) LIMIT 1
	`, artistID, title)
	al, err := scanAlbum(row)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	return al, nil
}

func (s *Store) findTrackByPath(path string) (*Track, error) {
	row := s.db.QueryRow(`
		SELECT id, album_id, artist_id, title, path FROM tracks WHERE path = ?
	`, path)
	t, err := scanTrack(row)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

func (s *Store) upsertTrack(t Track) (*Track, error) {
	existing, err := s.findTrackByPath(t.Path)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		_, err := s.db.Exec(`
			UPDATE tracks SET album_id = ?, artist_id = ?, title = ? WHERE id = ?
		`, t.AlbumID, t.ArtistID, t.Title, existing.ID)
		if err != nil {
			return nil, fmt.Errorf("update track: %w", err)
		}
		existing.AlbumID = t.AlbumID
		existing.ArtistID = t.ArtistID
		existing.Title = t.Title
		return existing, nil
	}
	if t.ID == "" {
		t.ID = "tr_" + uuid.NewString()[:8]
	}
	_, err = s.db.Exec(`
		INSERT INTO tracks (id, album_id, artist_id, title, path)
		VALUES (?, ?, ?, ?, ?)
	`, t.ID, t.AlbumID, t.ArtistID, t.Title, t.Path)
	if err != nil {
		return nil, fmt.Errorf("insert track: %w", err)
	}
	out := t
	return &out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanArtist(row rowScanner) (*Artist, error) {
	var a Artist
	var monitored int
	if err := row.Scan(&a.ID, &a.Name, &a.MusicBrainzID, &monitored, &a.Path); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("artist not found")
		}
		return nil, err
	}
	a.Monitored = monitored != 0
	return &a, nil
}

func scanAlbum(row rowScanner) (*Album, error) {
	var al Album
	var monitored int
	if err := row.Scan(&al.ID, &al.ArtistID, &al.Title, &al.MusicBrainzID, &al.Year, &monitored); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("album not found")
		}
		return nil, err
	}
	al.Monitored = monitored != 0
	return &al, nil
}

func scanTrack(row rowScanner) (*Track, error) {
	var t Track
	if err := row.Scan(&t.ID, &t.AlbumID, &t.ArtistID, &t.Title, &t.Path); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("track not found")
		}
		return nil, err
	}
	return &t, nil
}
