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
	"time"

	"google.golang.org/grpc"

	automationv1 "github.com/Muxcore-Media/contracts-automation/muxcore/automation/v1"
	mediaadminv1 "github.com/Muxcore-Media/contracts-media-admin/gen/muxcore/media/admin/v1"
	"github.com/Muxcore-Media/core/pkg/contracts"
	"github.com/Muxcore-Media/core/sdk/go/client"
	musicv1 "github.com/Muxcore-Media/media-music/proto/gen/muxcore/music/v1"
)

type Module struct {
	grpcLis             net.Listener
	httpLis             net.Listener
	grpcSrv             *grpc.Server
	mc                  *client.Client
	store               *Store
	httpSrv             *http.Server
	automationSearchFn  func(ctx context.Context, req *automationv1.SearchItemRequest) (*automationv1.SearchItemResponse, error)
	musicBrainzOverride musicBrainzAPI
	id                  string
	dataDir             string
	libraryDir          string
	imageDir            string
	grpcAddr            string
	httpAddr            string
	mu                  sync.RWMutex
	cfgMu               sync.RWMutex
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
	if v := os.Getenv("MUSIC_GRPC_ADDR"); v != "" {
		cfg.GRPCAddr = v
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
	imageDir := os.Getenv("MUSIC_IMAGE_DIR")
	if imageDir == "" {
		imageDir = filepath.Join(cfg.DataDir, "images")
	}
	return &Module{
		id: cfg.ID, grpcAddr: cfg.GRPCAddr, httpAddr: cfg.HTTPAddr,
		dataDir: cfg.DataDir, libraryDir: cfg.LibraryDir, imageDir: imageDir,
	}
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID: m.id, Name: "Music Manager", Version: "0.3.0",
		Roles:        []string{"media_manager", "media", "music"},
		Description:  "Lidarr-class music library manager with SQLite persistence and admin UI integration",
		Author:       "MuxCore",
		Capabilities: []string{"media.library", "media.library.music", "media.music", "music", "settings", "backupable"},
		Contracts: []contracts.ContractDeclaration{
			{Repo: "github.com/Muxcore-Media/contracts-media-admin", Interface: "MediaAdminService", Version: "v0.1.0"},
		},
		MinCoreVersion: "0.4.0",
		HTTPAddr:       m.httpAddr,
	}
}

func (m *Module) Init(ctx context.Context) error {
	if err := os.MkdirAll(m.dataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := os.MkdirAll(m.libraryDir, 0o700); err != nil {
		return fmt.Errorf("create library dir: %w", err)
	}
	if err := os.MkdirAll(m.getImageDir(), 0o700); err != nil {
		return fmt.Errorf("create image dir: %w", err)
	}
	dbPath := filepath.Join(m.dataDir, "music.db")
	store, err := OpenStore(ctx, dbPath)
	if err != nil {
		return err
	}
	m.store = store

	lc := net.ListenConfig{}
	lis, err := lc.Listen(ctx, "tcp", m.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen gRPC %s: %w", m.grpcAddr, err)
	}
	m.grpcLis = lis
	m.grpcAddr = lis.Addr().String()

	httpLis, err := lc.Listen(ctx, "tcp", m.httpAddr)
	if err != nil {
		return fmt.Errorf("listen HTTP %s: %w", m.httpAddr, err)
	}
	m.httpLis = httpLis
	m.httpAddr = httpLis.Addr().String()

	slog.Info("music library store open", "db", dbPath, "library", m.libraryDir, "grpc", m.grpcAddr, "http", m.httpAddr)
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	if m.store == nil {
		return fmt.Errorf("store not initialized")
	}
	m.grpcSrv = grpc.NewServer()
	mediaadminv1.RegisterMediaAdminServiceServer(m.grpcSrv, mediaAdminServer{m: m})
	musicv1.RegisterMusicManagementServiceServer(m.grpcSrv, &musicServer{m: m})
	registerMusicMesh(m.grpcSrv, m.id, m)

	go func() {
		slog.Info("music gRPC listening", "addr", m.grpcAddr)
		if err := m.grpcSrv.Serve(m.grpcLis); err != nil {
			slog.Error("gRPC serve", "error", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	m.registerMusicHTTPAPI(mux)
	mux.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/images/", http.FileServer(http.Dir(m.getImageDir()))).ServeHTTP(w, r)
	})
	m.httpSrv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("health listening", "addr", m.httpAddr)
		if err := m.httpSrv.Serve(m.httpLis); err != nil && err != http.ErrServerClosed {
			slog.Error("health serve", "error", err)
		}
	}()

	go m.runStartupScan(ctx)
	go m.dialCore(ctx)
	go m.subscribeToFileImported(ctx)
	return nil
}

func (m *Module) runStartupScan(ctx context.Context) {
	if res, err := m.ScanLibrary(ctx); err != nil {
		slog.Warn("music library scan on start", "error", err)
	} else {
		slog.Info("music library scan on start", "found", res.FilesFound, "imported", res.FilesImported, "skipped", res.FilesSkipped)
	}
}

func (m *Module) dialCore(ctx context.Context) {
	meshAddr := os.Getenv("MUXCORE_GRPC_ADDR")
	if meshAddr == "" {
		meshAddr = "localhost:9090"
	}
	insecureMode := os.Getenv("MUXCORE_INSECURE_DISABLE_TLS") == "true" || os.Getenv("MUXCORE_GRPC_INSECURE") == "true"
	var opts []client.Option
	if insecureMode {
		opts = append(opts, client.WithInsecure())
	}
	c, err := client.Dial(meshAddr, opts...)
	if err != nil {
		slog.Error("media-music: dial core", "error", err)
		return
	}
	m.mu.Lock()
	m.mc = c
	m.mu.Unlock()
	slog.Info("media-music: connected to core mesh", "addr", meshAddr)
}

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

func (m *Module) getLibraryDir() string {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	return m.libraryDir
}

// ScanLibrary scans the configured library root into SQLite.
func (m *Module) ScanLibrary(ctx context.Context) (*ScanResult, error) {
	m.cfgMu.RLock()
	root := m.libraryDir
	store := m.store
	m.cfgMu.RUnlock()
	if store == nil {
		return nil, fmt.Errorf("store not open")
	}
	return store.ScanLibraryRoot(ctx, root)
}

func (m *Module) GRPCListenAddr() string { return m.grpcAddr }
func (m *Module) HTTPListenAddr() string { return m.httpAddr }
