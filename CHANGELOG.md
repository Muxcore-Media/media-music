# Changelog

## [Unreleased]

### Security
- Track streaming, deletes, lyrics, imports, and artist paths must resolve inside `MUSIC_LIBRARY_DIR`. Symlink escapes are refused. An artist folder outside the library is no longer a second allow-list root.

## [0.3.2] - 2026-10-05

### Changed
- Built on core v0.6.14 / sdk/go/module v0.6.4: unregisters on shutdown and re-registers after core restarts (ADR-0022).

## [0.3.1] - 2026-10-05


### Changed
- Reported version comes from muxcore.json (ADR-0021); built on core v0.6.12 / sdk/go/module v0.6.3 (mesh enrollment, ADR-0017).

## [0.3.0] - 2026-10-05

### Changed
- Dependencies resolve from published GitHub tags (core v0.6.2, metadata-musicbrainz v0.1.0); no filesystem `replace`.
- CI runs on GitHub-hosted runners from the umbrella template; legacy workflow directory removed.

### Fixed
- Artwork download loop detects EOF with `errors.Is`.

## [v0.3.0] — 2026-08-31

### Added
- `MediaAdminService` (`contracts-media-admin`): list/get/update/delete/refresh, missing albums, tags, history, `SearchIndexers`
- MusicBrainz metadata sync on `AddArtist` / `RefreshMetadata` (name → `SearchArtist`, persist MBID, `EnsureAlbum` release groups)
- ModuleMesh `ScanLibrary`; `POST /api/scan`; scan on `Start` and after `library_dir` settings update
- HTTP `GET /api/tracks/{id}/stream` and `/lyrics` (library path sandbox)
- File-import event subscription with core dial retry; path guards on track/artist file delete
- Tags, missing albums, history tables; `PRAGMA foreign_keys=ON`; cascade delete cleans `item_tags`
- Peer gRPC dials honor `MUXCORE_INSECURE_DISABLE_TLS` (metadata-musicbrainz, media.automation)
- CI `golangci-lint` job; Dockerfile copies replace siblings for workspace builds

### Changed
- `Info().HTTPAddr` reports bound HTTP listen address after `Init`
- Version / `muxcore.json` → 0.3.0; documents `MUSIC_*` env vars and RPC/HTTP surface

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
