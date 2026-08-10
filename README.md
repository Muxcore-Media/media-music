# Media Music

Lidarr-class **music library manager** scaffold for MuxCore.

Exposes `muxcore.music.v1.MusicManagementService` (artists/albums CRUD) with an in-memory store in **v0.1.0**, plus SettingsProvider for `library_dir`.

## Ports

| Service | Default |
|---------|---------|
| gRPC | `:9640` |
| Health | `:9641` (`/healthz`) |

## Build / test

```bash
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go build -o bin/media-music ./cmd/module
```

## Status

v0.1.0 scaffold — optional peer (not default host). Persistence, MusicBrainz search, and acquisition wiring are follow-ups.
