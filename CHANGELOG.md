# Changelog

## [v0.2.1] — 2026-08-10

### Added
- ModuleMesh methods `ListArtists` / `GetArtist` (JSON) for admin-ui soft proxy without MusicManagementService client
- HTTP JSON stubs `GET /api/artists` and `GET /api/artists/{id}` on health port
- Settings mesh methods retained alongside library mesh methods

## [v0.2.0] — 2026-08-10

### Added
- SQLite persistence for artists, albums, and tracks (`$MUSIC_DATA_DIR/music.db`)
- Library root scanner (path-derived metadata only; no network APIs)
- Offline scan fixtures under `internal/testdata/library`
- Self-hosted GitHub Actions CI

### Changed
- Replaced in-memory store with `modernc.org/sqlite` (WAL)

## [v0.1.0] — 2026-08-10

### Added
- `MusicManagementService` (Add/Get/List/Remove artist; Add/List album)
- In-memory library store
- SettingsProvider (`library_dir`)
- Health `:9641`
