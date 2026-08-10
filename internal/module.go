package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"

	"google.golang.org/grpc"

	"github.com/Muxcore-Media/core/pkg/contracts"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
	musicv1 "github.com/Muxcore-Media/media-music/proto/gen/muxcore/music/v1"
)

type Module struct {
	id         string
	grpcAddr   string
	httpAddr   string
	libraryDir string

	cfgMu sync.RWMutex
	store *Store

	grpcSrv *grpc.Server
	lis     net.Listener
	httpSrv *http.Server
}

type Config struct {
	ID         string
	LibraryDir string
	GRPCAddr   string
	HTTPAddr   string
}

func NewModule(cfg Config) *Module {
	if cfg.ID == "" {
		cfg.ID = "media-music"
	}
	if cfg.GRPCAddr == "" {
		cfg.GRPCAddr = ":9640"
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":9641"
	}
	if v := os.Getenv("MUSIC_LIBRARY_DIR"); v != "" {
		cfg.LibraryDir = v
	}
	if v := os.Getenv("MUXCORE_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if cfg.LibraryDir == "" {
		cfg.LibraryDir = "./data/music"
	}
	return &Module{
		id: cfg.ID, grpcAddr: cfg.GRPCAddr, httpAddr: cfg.HTTPAddr,
		libraryDir: cfg.LibraryDir, store: NewStore(),
	}
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID: m.id, Name: "Music Manager", Version: "0.1.0",
		Roles:        []string{"media", "music"},
		Description:  "Lidarr-class music library manager (scaffold)",
		Capabilities: []string{"media.music", "music", "settings"},
		HTTPAddr:     m.grpcAddr,
	}
}

func (m *Module) Init(ctx context.Context) error { return nil }

func (m *Module) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", m.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen gRPC %s: %w", m.grpcAddr, err)
	}
	m.lis = lis
	m.grpcSrv = grpc.NewServer()
	musicv1.RegisterMusicManagementServiceServer(m.grpcSrv, &musicServer{m: m})
	modulesdk.RegisterSettings(m.grpcSrv, m.id, m)
	go func() {
		slog.Info("music gRPC listening", "addr", m.grpcAddr)
		if err := m.grpcSrv.Serve(lis); err != nil {
			slog.Error("gRPC serve", "error", err)
		}
	}()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	m.httpSrv = &http.Server{Addr: m.httpAddr, Handler: mux}
	go func() {
		slog.Info("health listening", "addr", m.httpAddr)
		if err := m.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health serve", "error", err)
		}
	}()
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m.grpcSrv != nil {
		m.grpcSrv.GracefulStop()
	}
	if m.httpSrv != nil {
		_ = m.httpSrv.Shutdown(ctx)
	}
	return nil
}

func (m *Module) Health(ctx context.Context) error { return nil }

type musicServer struct {
	musicv1.UnimplementedMusicManagementServiceServer
	m *Module
}

func (s *musicServer) AddArtist(_ context.Context, req *musicv1.AddArtistRequest) (*musicv1.AddArtistResponse, error) {
	a, err := s.m.store.AddArtist(Artist{
		Name: req.GetName(), MusicBrainzID: req.GetMusicbrainzId(),
		Monitored: req.GetMonitored(), Path: req.GetPath(),
	})
	if err != nil {
		return nil, err
	}
	return &musicv1.AddArtistResponse{Artist: toPBArtist(a)}, nil
}

func (s *musicServer) GetArtist(_ context.Context, req *musicv1.GetArtistRequest) (*musicv1.GetArtistResponse, error) {
	a, err := s.m.store.GetArtist(req.GetId())
	if err != nil {
		return nil, err
	}
	return &musicv1.GetArtistResponse{Artist: toPBArtist(a)}, nil
}

func (s *musicServer) ListArtists(_ context.Context, req *musicv1.ListArtistsRequest) (*musicv1.ListArtistsResponse, error) {
	items := s.m.store.ListArtists(req.GetQuery())
	out := make([]*musicv1.Artist, 0, len(items))
	for _, a := range items {
		out = append(out, toPBArtist(a))
	}
	return &musicv1.ListArtistsResponse{Artists: out}, nil
}

func (s *musicServer) RemoveArtist(_ context.Context, req *musicv1.RemoveArtistRequest) (*musicv1.RemoveArtistResponse, error) {
	if err := s.m.store.RemoveArtist(req.GetId()); err != nil {
		return nil, err
	}
	return &musicv1.RemoveArtistResponse{Success: true}, nil
}

func (s *musicServer) AddAlbum(_ context.Context, req *musicv1.AddAlbumRequest) (*musicv1.AddAlbumResponse, error) {
	al, err := s.m.store.AddAlbum(Album{
		ArtistID: req.GetArtistId(), Title: req.GetTitle(),
		MusicBrainzID: req.GetMusicbrainzId(), Year: req.GetYear(), Monitored: req.GetMonitored(),
	})
	if err != nil {
		return nil, err
	}
	return &musicv1.AddAlbumResponse{Album: toPBAlbum(al)}, nil
}

func (s *musicServer) ListAlbums(_ context.Context, req *musicv1.ListAlbumsRequest) (*musicv1.ListAlbumsResponse, error) {
	items := s.m.store.ListAlbums(req.GetArtistId())
	out := make([]*musicv1.Album, 0, len(items))
	for _, a := range items {
		out = append(out, toPBAlbum(a))
	}
	return &musicv1.ListAlbumsResponse{Albums: out}, nil
}

func toPBArtist(a *Artist) *musicv1.Artist {
	return &musicv1.Artist{
		Id: a.ID, Name: a.Name, MusicbrainzId: a.MusicBrainzID,
		Monitored: a.Monitored, Path: a.Path,
	}
}

func toPBAlbum(a *Album) *musicv1.Album {
	return &musicv1.Album{
		Id: a.ID, ArtistId: a.ArtistID, Title: a.Title,
		MusicbrainzId: a.MusicBrainzID, Year: a.Year, Monitored: a.Monitored,
	}
}
