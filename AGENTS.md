# AGENTS.md — media-music

MuxCore sidecar module (`media-music`). Workspace deploy and SSH: [`../AGENTS.md`](../AGENTS.md). Default ports: [`_mvp/PORTS.md`](../_mvp/PORTS.md).

## Module identity

| Field | Value |
|-------|-------|
| Directory | `media-music` |
| Version | 0.3.0 |
| Capabilities | see `muxcore.json` |
| Contracts | `contracts-media-admin` → `MediaAdminService` v0.1.0 |

## Agent rules

- Modules run as gRPC sidecars; capabilities are the security boundary.
- TLS required in production (`MUXCORE_INSECURE_DISABLE_TLS` is dev-only).
- Match existing Go patterns; run `gofmt` and package tests before finishing.
- Cross-module events: prefer `github.com/Muxcore-Media/contracts-media/events` over deprecated `core/pkg/contracts` aliases.
- Do not edit polluted workspace dumps (see `MASTER-ROADMAP.md` Appendix H).
- MusicBrainz and automation clients use `meshGRPCDialOpts()` — same TLS/insecure path as core dial.

## Build

```bash
cd media-music
export GOCACHE=/tmp/gocache-media-music
go test ./...
golangci-lint run ./...
```

## Key paths

| Area | Location |
|------|----------|
| Music RPC | `proto/muxcore/music/v1/music.proto`, `internal/grpc_music.go` |
| MediaAdmin | `internal/media_admin.go`, `internal/media_admin_server.go` |
| MusicBrainz sync | `internal/metadata_client.go`, `internal/metadata_sync.go` |
| HTTP API | `internal/http_api.go` |
| Mesh | `internal/mesh.go` |
| Store | `internal/store.go`, `internal/store_helpers.go` |

## Env (module-specific)

- `MUSIC_DATA_DIR`, `MUSIC_LIBRARY_DIR`, `MUSIC_GRPC_ADDR`
- `MUXCORE_HTTP_ADDR` — HTTP/health + JSON API bind
