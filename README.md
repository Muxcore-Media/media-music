# Media Music

Lidarr-class **music library manager** for MuxCore.

Exposes `muxcore.music.v1.MusicManagementService` (artists/albums CRUD) with **SQLite persistence** under the data directory, plus SettingsProvider for `library_dir` / `data_dir`.

Admin-ui soft-proxies the library via ModuleMesh methods `ListArtists` / `GetArtist` (same pattern as Settings) and optional HTTP stubs on the health port:

| Path | Purpose |
|------|---------|
| `GET /api/artists?q=` | JSON artist list |
| `GET /api/artists/{id}` | JSON artist + albums |

Library scanning walks the configured root for local audio stubs (`.mp3`, `.flac`, …) and upserts artists/albums/tracks from the path layout `Artist/Album/track.ext`. Metadata is derived from filenames only — **no paid or network metadata APIs**.

## Ports

| Service | Default |
|---------|---------|
| gRPC | `:9640` |
| Health | `:9641` (`/healthz`) |

## Data layout

| Path | Purpose |
|------|---------|
| `$MUSIC_DATA_DIR/music.db` (default `./data/music.db`) | SQLite library |
| `$MUSIC_LIBRARY_DIR` (default `./data/music`) | Audio files to scan |

## Build / test

```bash
export PATH=$HOME/.local/go/bin:$PATH
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go build -o bin/media-music ./cmd/module
```

Offline tests use `internal/testdata/library` (empty `.mp3` / `.flac` stubs).

## Status

v0.2.1 — SQLite + scan fixtures; admin mesh/HTTP list stubs. MusicBrainz search and acquisition wiring remain follow-ups.
