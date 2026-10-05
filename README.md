# Media Music

Lidarr-class **music library manager** for MuxCore (v0.3.0).

SQLite-backed artists/albums/tracks, MusicBrainz metadata sync via `metadata-musicbrainz`, MediaAdmin integration, operator scan/stream/lyrics HTTP, and mesh soft-proxy for admin-ui.

## Services

| Service | Default | Notes |
|---------|---------|-------|
| gRPC (`MusicManagementService`, `MediaAdminService`, `ModuleMesh`) | `:9640` | `MUSIC_GRPC_ADDR` |
| HTTP (health + JSON API) | `:9641` | `MUXCORE_HTTP_ADDR`; `Info().HTTPAddr` is the bound listen address |

## Environment

| Variable | Purpose |
|----------|---------|
| `MUSIC_DATA_DIR` | SQLite + module data (default `./data`) |
| `MUSIC_LIBRARY_DIR` | Library root scanned for audio (default `$MUSIC_DATA_DIR/music`) |
| `MUSIC_GRPC_ADDR` | gRPC bind address |
| `MUXCORE_HTTP_ADDR` | HTTP bind address |
| `MUXCORE_GRPC_ADDR` | Core mesh dial target (file-import events, discovery) |
| `MUXCORE_INSECURE_DISABLE_TLS` / `MUXCORE_GRPC_INSECURE` | Dev-only plaintext gRPC to core and peer modules |
| `MUXCORE_MESH_DIAL_LOCAL` | Dial peer modules on `127.0.0.1:<port>` when HTTP addr is wildcard |

## gRPC

### `muxcore.music.v1.MusicManagementService`

Artists, albums, track files, tags, missing albums, metadata refresh. Key RPCs: `AddArtist`, `RemoveArtist`, `RefreshMetadata`, `ListMissing`, `AddTrackFile`, `RemoveTrackFile`, `CreateTag`, `ListTags`, `SetItemTags`.

### `muxcore.media.admin.v1.MediaAdminService`

Declared in `muxcore.json`. Features: **missing**, **tags**. Implements list/get/update/delete/refresh, artwork list (poster URL when set), history, and `SearchIndexers` (via `media.automation` capability or injectable test override).

### ModuleMesh

| Method | Purpose |
|--------|---------|
| `Settings` / `UpdateSetting` | `library_dir`, `data_dir` |
| `ListArtists` / `GetArtist` | JSON library list/detail for admin-ui |
| `ScanLibrary` | Walk `library_dir` into SQLite |

## HTTP (health port)

| Path | Purpose |
|------|---------|
| `GET /healthz` | Liveness |
| `GET /api/artists?q=` | JSON artist list |
| `GET /api/artists/{id}` | JSON artist + albums + tracks |
| `POST /api/scan` | Trigger library scan |
| `GET /api/tracks/{id}/stream` | Serve audio (path confined to `library_dir` / artist path) |
| `GET /api/tracks/{id}/lyrics` | Sidecar `.lrc`/`.txt` JSON (no filesystem path in response) |

Library layout: `Artist/Album/track.ext`. Scan and `EventFileImported` upsert from paths/filenames; MusicBrainz fills MBID + release groups when no MBID is set on add/refresh.

## Data layout

| Path | Purpose |
|------|---------|
| `$MUSIC_DATA_DIR/music.db` | SQLite library (WAL, foreign keys ON) |
| `$MUSIC_LIBRARY_DIR` | Audio files |

## Build / test

```bash
cd media-music
export GOCACHE=/tmp/gocache-media-music
go test ./...
CGO_ENABLED=0 go build -o bin/media-music ./cmd/module
```

Offline fixtures: `internal/testdata/library` (empty audio stubs + `.lrc`).

Docker (from workspace root): `docker build -f media-music/Dockerfile .`

## Status

v0.3.0 — MediaAdmin, MusicBrainz name lookup + release groups, library scan (start/settings/mesh/HTTP), file-import retry, path guards, stream/lyrics, tags/missing/history, golangci-lint CI.
