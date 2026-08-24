package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Artist struct {
	ID               string
	Name             string
	MusicBrainzID    string
	Path             string
	QualityProfileID string
	RootFolderPath   string
	Overview         string
	PosterURL        string
	CreatedAt        string
	UpdatedAt        string
	Monitored        bool
	HasFile          bool
}

type Album struct {
	ID            string
	ArtistID      string
	Title         string
	MusicBrainzID string
	Year          int32
	Monitored     bool
	HasFile       bool
}

// Track is a single audio file in the library.
type Track struct {
	ID        string
	AlbumID   string
	ArtistID  string
	Title     string
	Path      string
	Quality   string
	Container string
	CreatedAt string
	SizeBytes int64
}

type Tag struct {
	ID        string
	Label     string
	CreatedAt string
}

type MissingAlbum struct {
	AlbumID          string
	ArtistID         string
	ArtistName       string
	Title            string
	MusicBrainzID    string
	QualityProfileID string
	RootFolderPath   string
	Year             int32
}

// Store persists the music library in SQLite.
type Store struct {
	db *sql.DB
}

// OpenStore opens or creates the SQLite database at path (WAL mode).
func OpenStore(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
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
		CREATE TABLE IF NOT EXISTS tags (
			id TEXT PRIMARY KEY,
			label TEXT UNIQUE NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS item_tags (
			item_id TEXT NOT NULL,
			tag_id TEXT NOT NULL,
			PRIMARY KEY (item_id, tag_id)
		);
		CREATE TABLE IF NOT EXISTS history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			source_title TEXT DEFAULT '',
			quality TEXT DEFAULT '',
			data TEXT DEFAULT '{}',
			date TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_history_item ON history(item_id);
	`)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	for _, col := range []string{
		`ALTER TABLE artists ADD COLUMN has_file INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE artists ADD COLUMN quality_profile_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE artists ADD COLUMN root_folder_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE artists ADD COLUMN overview TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE artists ADD COLUMN poster_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE artists ADD COLUMN created_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE artists ADD COLUMN updated_at TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE albums ADD COLUMN has_file INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE tracks ADD COLUMN quality TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tracks ADD COLUMN size_bytes INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE tracks ADD COLUMN container TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE tracks ADD COLUMN created_at TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.ExecContext(ctx, col); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("migrate column: %w", err)
		}
	}
	return nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) AddArtist(ctx context.Context, a Artist) (*Artist, error) {
	if strings.TrimSpace(a.Name) == "" {
		return nil, fmt.Errorf("artist name required")
	}
	if a.ID == "" {
		a.ID = "ar_" + uuid.NewString()[:8]
	}
	now := nowRFC3339()
	if a.CreatedAt == "" {
		a.CreatedAt = now
	}
	if a.UpdatedAt == "" {
		a.UpdatedAt = now
	}
	monitored := 0
	if a.Monitored {
		monitored = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO artists (
			id, name, musicbrainz_id, monitored, path, has_file,
			quality_profile_id, root_folder_path, overview, poster_url, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?)
	`, a.ID, a.Name, a.MusicBrainzID, monitored, a.Path,
		a.QualityProfileID, a.RootFolderPath, a.Overview, a.PosterURL, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert artist: %w", err)
	}
	out := a
	return &out, nil
}

func (s *Store) GetArtist(ctx context.Context, id string) (*Artist, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, musicbrainz_id, monitored, path, has_file,
			quality_profile_id, root_folder_path, overview, poster_url, created_at, updated_at
		FROM artists WHERE id = ?
	`, id)
	return scanArtist(row)
}

func (s *Store) ListArtists(ctx context.Context, query string) ([]*Artist, error) {
	items, _, err := s.ListArtistsPage(ctx, query, "", 1, 0, "", "")
	return items, err
}

func (s *Store) ListArtistsPage(ctx context.Context, query, tagID string, page, pageSize int, sortBy, sortOrder string) ([]*Artist, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	if sortBy == "" {
		sortBy = "name"
	}
	if sortOrder != "desc" {
		sortOrder = "asc"
	}
	validSortBy := map[string]bool{
		"name": true, "created_at": true, "updated_at": true, "path": true,
	}
	if !validSortBy[sortBy] {
		sortBy = "name"
	}
	base := `FROM artists`
	var where []string
	var args []any
	if q := strings.TrimSpace(query); q != "" {
		where = append(where, `name LIKE ?`)
		args = append(args, "%"+q+"%")
	}
	if tagID != "" {
		where = append(where, `id IN (SELECT item_id FROM item_tags WHERE tag_id = ?)`)
		args = append(args, tagID)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) `+base+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count artists: %w", err)
	}
	offset := (page - 1) * pageSize
	//nolint:gosec // sortBy is validated against allowlist before use in ORDER BY
	q := `SELECT id, name, musicbrainz_id, monitored, path, has_file,
		quality_profile_id, root_folder_path, overview, poster_url, created_at, updated_at ` +
		base + clause + fmt.Sprintf(` ORDER BY %s %s LIMIT ? OFFSET ?`, sortBy, sortOrder)
	qargs := append(append([]any{}, args...), pageSize, offset)
	rows, err := s.db.QueryContext(ctx, q, qargs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list artists: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]*Artist, 0)
	for rows.Next() {
		a, err := scanArtist(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (s *Store) RemoveArtist(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM artists WHERE id = ?`, id)
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
	_, _ = s.db.ExecContext(ctx, `DELETE FROM tracks WHERE artist_id = ?`, id)
	_, _ = s.db.ExecContext(ctx, `DELETE FROM albums WHERE artist_id = ?`, id)
	return nil
}

func (s *Store) AddAlbum(ctx context.Context, al Album) (*Album, error) {
	if strings.TrimSpace(al.Title) == "" {
		return nil, fmt.Errorf("album title required")
	}
	var exists string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM artists WHERE id = ?`, al.ArtistID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
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
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO albums (id, artist_id, title, musicbrainz_id, year, monitored)
		VALUES (?, ?, ?, ?, ?, ?)
	`, al.ID, al.ArtistID, al.Title, al.MusicBrainzID, al.Year, monitored)
	if err != nil {
		return nil, fmt.Errorf("insert album: %w", err)
	}
	out := al
	return &out, nil
}

func (s *Store) ListAlbums(ctx context.Context, artistID string) ([]*Album, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if artistID != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, artist_id, title, musicbrainz_id, year, monitored, has_file
			FROM albums WHERE artist_id = ? ORDER BY year, title
		`, artistID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, artist_id, title, musicbrainz_id, year, monitored, has_file
			FROM albums ORDER BY title
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("list albums: %w", err)
	}
	defer func() { _ = rows.Close() }()
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

// ListTracks returns tracks, optionally filtered by album or artist ID.
func (s *Store) ListTracks(ctx context.Context, albumID, artistID string) ([]*Track, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if albumID != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, album_id, artist_id, title, path, quality, size_bytes, container, created_at
			FROM tracks WHERE album_id = ? ORDER BY title
		`, albumID)
	} else if artistID != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, album_id, artist_id, title, path, quality, size_bytes, container, created_at
			FROM tracks WHERE artist_id = ? ORDER BY title
		`, artistID)
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, album_id, artist_id, title, path, quality, size_bytes, container, created_at
			FROM tracks ORDER BY title
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("list tracks: %w", err)
	}
	defer func() { _ = rows.Close() }()
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

func (s *Store) findAlbum(ctx context.Context, artistID, title string) (*Album, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, artist_id, title, musicbrainz_id, year, monitored, has_file FROM albums
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

func (s *Store) findTrackByPath(ctx context.Context, path string) (*Track, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, album_id, artist_id, title, path, quality, size_bytes, container, created_at FROM tracks WHERE path = ?
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

func (s *Store) upsertTrack(ctx context.Context, t Track) (*Track, error) {
	existing, err := s.findTrackByPath(ctx, t.Path)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		_, execErr := s.db.ExecContext(ctx, `
			UPDATE tracks SET album_id = ?, artist_id = ?, title = ?, quality = ?, size_bytes = ?, container = ? WHERE id = ?
		`, t.AlbumID, t.ArtistID, t.Title, t.Quality, t.SizeBytes, t.Container, existing.ID)
		if execErr != nil {
			return nil, fmt.Errorf("update track: %w", execErr)
		}
		existing.AlbumID = t.AlbumID
		existing.ArtistID = t.ArtistID
		existing.Title = t.Title
		existing.Quality = t.Quality
		existing.SizeBytes = t.SizeBytes
		existing.Container = t.Container
		_ = s.refreshAlbumHasFile(ctx, t.AlbumID)
		_ = s.refreshArtistHasFile(ctx, t.ArtistID)
		return existing, nil
	}
	if t.ID == "" {
		t.ID = "tr_" + uuid.NewString()[:8]
	}
	if t.CreatedAt == "" {
		t.CreatedAt = nowRFC3339()
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO tracks (id, album_id, artist_id, title, path, quality, size_bytes, container, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.AlbumID, t.ArtistID, t.Title, t.Path, t.Quality, t.SizeBytes, t.Container, t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert track: %w", err)
	}
	_ = s.refreshAlbumHasFile(ctx, t.AlbumID)
	_ = s.refreshArtistHasFile(ctx, t.ArtistID)
	out := t
	return &out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanArtist(row rowScanner) (*Artist, error) {
	var a Artist
	var monitored, hasFile int
	if err := row.Scan(
		&a.ID, &a.Name, &a.MusicBrainzID, &monitored, &a.Path, &hasFile,
		&a.QualityProfileID, &a.RootFolderPath, &a.Overview, &a.PosterURL, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("artist not found")
		}
		return nil, err
	}
	a.Monitored = monitored != 0
	a.HasFile = hasFile != 0
	return &a, nil
}

func scanAlbum(row rowScanner) (*Album, error) {
	var al Album
	var monitored, hasFile int
	if err := row.Scan(&al.ID, &al.ArtistID, &al.Title, &al.MusicBrainzID, &al.Year, &monitored, &hasFile); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("album not found")
		}
		return nil, err
	}
	al.Monitored = monitored != 0
	al.HasFile = hasFile != 0
	return &al, nil
}

func scanTrack(row rowScanner) (*Track, error) {
	var t Track
	if err := row.Scan(
		&t.ID, &t.AlbumID, &t.ArtistID, &t.Title, &t.Path,
		&t.Quality, &t.SizeBytes, &t.Container, &t.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("track not found")
		}
		return nil, err
	}
	return &t, nil
}
