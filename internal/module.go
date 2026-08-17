package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/grpc"

	"github.com/Muxcore-Media/core/pkg/contracts"
	musicv1 "github.com/Muxcore-Media/media-music/proto/gen/muxcore/music/v1"
)

type Module struct {
	id         string
	grpcAddr   string
	httpAddr   string
	dataDir    string
	libraryDir string

	cfgMu sync.RWMutex
	store *Store

	grpcSrv *grpc.Server
	lis     net.Listener
	httpSrv *http.Server
}

type Config struct {
	ID         string
	DataDir    string
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
	if v := os.Getenv("MUSIC_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("MUSIC_LIBRARY_DIR"); v != "" {
		cfg.LibraryDir = v
	}
	if v := os.Getenv("MUXCORE_HTTP_ADDR"); v != "" {
		cfg.HTTPAddr = v
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	if cfg.LibraryDir == "" {
		cfg.LibraryDir = filepath.Join(cfg.DataDir, "music")
	}
	return &Module{
		id: cfg.ID, grpcAddr: cfg.GRPCAddr, httpAddr: cfg.HTTPAddr,
		dataDir: cfg.DataDir, libraryDir: cfg.LibraryDir,
	}
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID: m.id, Name: "Music Manager", Version: "0.2.1",
		Roles:        []string{"media", "music"},
		Description:  "Lidarr-class music library manager with SQLite persistence",
		Capabilities: []string{"media.music", "music", "settings"},
		HTTPAddr:     m.grpcAddr,
	}
}

func (m *Module) Init(ctx context.Context) error {
	if err := os.MkdirAll(m.dataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(m.libraryDir, 0o700); err != nil {
		return fmt.Errorf("create library dir: %w", err)
	}
	dbPath := filepath.Join(m.dataDir, "music.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		return err
	}
	m.store = store
	slog.Info("music library store open", "db", dbPath, "library", m.libraryDir)
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	if m.store == nil {
		return fmt.Errorf("store not initialized")
	}
	lis, err := net.Listen("tcp", m.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen gRPC %s: %w", m.grpcAddr, err)
	}
	m.lis = lis
	m.grpcAddr = lis.Addr().String()
	m.grpcSrv = grpc.NewServer()
	musicv1.RegisterMusicManagementServiceServer(m.grpcSrv, &musicServer{m: m})
	registerMusicMesh(m.grpcSrv, m.id, m)
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
	m.registerMusicHTTPAPI(mux)
	httpLis, err := net.Listen("tcp", m.httpAddr)
	if err != nil {
		return fmt.Errorf("listen HTTP %s: %w", m.httpAddr, err)
	}
	m.httpAddr = httpLis.Addr().String()
	m.httpSrv = &http.Server{Handler: mux}
	go func() {
		slog.Info("health listening", "addr", m.httpAddr)
		if err := m.httpSrv.Serve(httpLis); err != nil && err != http.ErrServerClosed {
			slog.Error("health serve", "error", err)
		}
	}()
	return nil
}

// GRPCListenAddr returns the bound gRPC address after Start (useful in tests).
func (m *Module) GRPCListenAddr() string { return m.grpcAddr }

// HTTPListenAddr returns the bound health/HTTP API address after Start.
func (m *Module) HTTPListenAddr() string { return m.httpAddr }

func (m *Module) Stop(ctx context.Context) error {
	if m.grpcSrv != nil {
		m.grpcSrv.GracefulStop()
	}
	if m.httpSrv != nil {
		_ = m.httpSrv.Shutdown(ctx)
	}
	if m.store != nil {
		_ = m.store.Close()
		m.store = nil
	}
	return nil
}

func (m *Module) Health(ctx context.Context) error {
	if m.store == nil {
		return fmt.Errorf("store not open")
	}
	return nil
}

// ScanLibrary scans the configured library root into SQLite.
func (m *Module) ScanLibrary() (*ScanResult, error) {
	m.cfgMu.RLock()
	root := m.libraryDir
	store := m.store
	m.cfgMu.RUnlock()
	if store == nil {
		return nil, fmt.Errorf("store not open")
	}
	return store.ScanLibraryRoot(root)
}

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
	items, err := s.m.store.ListArtists(req.GetQuery())
	if err != nil {
		return nil, err
	}
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
	items, err := s.m.store.ListAlbums(req.GetArtistId())
	if err != nil {
		return nil, err
	}
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
