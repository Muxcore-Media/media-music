# Build from MuxCore workspace root:
#   docker build -f media-music/Dockerfile .
FROM golang:1.26-alpine AS builder
COPY core/ /build/core/
COPY contracts-media-admin/ /build/contracts-media-admin/
COPY contracts-automation/ /build/contracts-automation/
COPY contracts-media/ /build/contracts-media/
COPY metadata-musicbrainz/ /build/metadata-musicbrainz/
COPY media-music/ /build/media-music/
WORKDIR /build/media-music
RUN go mod download && CGO_ENABLED=0 go build -o /media-music ./cmd/module
FROM alpine:3.21
RUN adduser -D -h /data app
USER app
WORKDIR /app
COPY --from=builder /media-music .
ENTRYPOINT ["./media-music"]
