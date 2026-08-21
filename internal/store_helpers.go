package internal

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *Store) refreshArtistHasFile(artistID string) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tracks WHERE artist_id = ?`, artistID).Scan(&n); err != nil {
		return err
	}
	has := 0
	if n > 0 {
		has = 1
	}
	_, err := s.db.Exec(`UPDATE artists SET has_file = ?, updated_at = ? WHERE id = ?`, has, nowRFC3339(), artistID)
	return err
}

func (s *Store) refreshAlbumHasFile(albumID string) error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tracks WHERE album_id = ?`, albumID).Scan(&n); err != nil {
		return err
	}
	has := 0
	if n > 0 {
		has = 1
	}
	_, err := s.db.Exec(`UPDATE albums SET has_file = ? WHERE id = ?`, has, albumID)
	return err
}

func (s *Store) UpdateArtist(id string, fields map[string]any) (*Artist, error) {
	a, err := s.GetArtist(id)
	if err != nil {
		return nil, err
	}
	if v, ok := fields["name"].(string); ok && strings.TrimSpace(v) != "" {
		a.Name = v
	}
	if v, ok := fields["overview"].(string); ok {
		a.Overview = v
	}
	if v, ok := fields["quality_profile_id"].(string); ok {
		a.QualityProfileID = v
	}
	if v, ok := fields["root_folder_path"].(string); ok {
		a.RootFolderPath = v
	}
	if v, ok := fields["monitored"].(bool); ok {
		a.Monitored = v
	}
	monitored := 0
	if a.Monitored {
		monitored = 1
	}
	a.UpdatedAt = nowRFC3339()
	_, err = s.db.Exec(`
		UPDATE artists SET name = ?, overview = ?, quality_profile_id = ?, root_folder_path = ?,
			monitored = ?, updated_at = ? WHERE id = ?
	`, a.Name, a.Overview, a.QualityProfileID, a.RootFolderPath, monitored, a.UpdatedAt, id)
	if err != nil {
		return nil, fmt.Errorf("update artist: %w", err)
	}
	return s.GetArtist(id)
}

func (s *Store) UpdateAlbumMonitored(albumID string, monitored bool) error {
	m := 0
	if monitored {
		m = 1
	}
	res, err := s.db.Exec(`UPDATE albums SET monitored = ? WHERE id = ?`, m, albumID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("album %q not found", albumID)
	}
	return nil
}

func (s *Store) GetTrack(id string) (*Track, error) {
	row := s.db.QueryRow(`
		SELECT id, album_id, artist_id, title, path, quality, size_bytes, container, created_at
		FROM tracks WHERE id = ?
	`, id)
	return scanTrack(row)
}

func (s *Store) RemoveTrack(id string, deleteFile bool) error {
	t, err := s.GetTrack(id)
	if err != nil {
		return err
	}
	if deleteFile && t.Path != "" {
		_ = os.Remove(t.Path)
	}
	if _, err := s.db.Exec(`DELETE FROM tracks WHERE id = ?`, id); err != nil {
		return err
	}
	_ = s.refreshAlbumHasFile(t.AlbumID)
	_ = s.refreshArtistHasFile(t.ArtistID)
	return nil
}

func (s *Store) AddTrackFile(albumID, filePath, title, quality, container string, size int64) (*Track, error) {
	al, err := s.findAlbumByID(albumID)
	if err != nil {
		return nil, err
	}
	if al == nil {
		return nil, fmt.Errorf("album %q not found", albumID)
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}
	return s.upsertTrack(Track{
		AlbumID: al.ID, ArtistID: al.ArtistID, Title: title, Path: filePath,
		Quality: quality, SizeBytes: size, Container: container,
	})
}

func (s *Store) findAlbumByID(id string) (*Album, error) {
	row := s.db.QueryRow(`
		SELECT id, artist_id, title, musicbrainz_id, year, monitored, has_file FROM albums WHERE id = ?
	`, id)
	return scanAlbum(row)
}

func (s *Store) ListMissing(page, pageSize int, artistID string) ([]*MissingAlbum, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}
	base := `
		FROM albums al
		INNER JOIN artists ar ON ar.id = al.artist_id
		WHERE al.monitored = 1 AND al.has_file = 0`
	args := []any{}
	if artistID != "" {
		base += ` AND al.artist_id = ?`
		args = append(args, artistID)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) `+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	q := `SELECT al.id, al.artist_id, ar.name, al.title, al.year, al.musicbrainz_id,
		ar.quality_profile_id, ar.root_folder_path ` + base +
		` ORDER BY ar.name, al.year, al.title LIMIT ? OFFSET ?`
	qargs := append(append([]any{}, args...), pageSize, offset)
	rows, err := s.db.Query(q, qargs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]*MissingAlbum, 0)
	for rows.Next() {
		var m MissingAlbum
		if err := rows.Scan(&m.AlbumID, &m.ArtistID, &m.ArtistName, &m.Title, &m.Year,
			&m.MusicBrainzID, &m.QualityProfileID, &m.RootFolderPath); err != nil {
			return nil, 0, err
		}
		out = append(out, &m)
	}
	return out, total, rows.Err()
}

func (s *Store) appendHistory(itemID, eventType, sourceTitle, quality string) {
	_, _ = s.db.Exec(`INSERT INTO history (item_id, event_type, source_title, quality, date) VALUES (?, ?, ?, ?, ?)`,
		itemID, eventType, sourceTitle, quality, nowRFC3339())
}

func (s *Store) findArtistByName(name string) (*Artist, error) {
	row := s.db.QueryRow(`
		SELECT id, name, musicbrainz_id, monitored, path, has_file,
			quality_profile_id, root_folder_path, overview, poster_url, created_at, updated_at
		FROM artists WHERE lower(name) = lower(?) LIMIT 1
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

func (s *Store) scanHistory(page, pageSize int, itemID, eventType string) ([]map[string]string, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	base := `FROM history`
	var where []string
	var args []any
	if itemID != "" {
		where = append(where, `item_id = ?`)
		args = append(args, itemID)
	}
	if eventType != "" {
		where = append(where, `event_type = ?`)
		args = append(args, eventType)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) `+base+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	rows, err := s.db.Query(`SELECT item_id, event_type, source_title, quality, date `+base+clause+
		` ORDER BY date DESC LIMIT ? OFFSET ?`, append(append([]any{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]map[string]string, 0)
	for rows.Next() {
		var itemID, ev, src, qual, date string
		if err := rows.Scan(&itemID, &ev, &src, &qual, &date); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]string{
			"item_id": itemID, "event_type": ev, "source_title": src, "quality": qual, "date": date,
		})
	}
	return out, total, rows.Err()
}

func nullableString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}
