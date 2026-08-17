package internal_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	meshv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/mesh/v1"
	"github.com/Muxcore-Media/media-music/internal"
)

func TestMeshAndHTTPListArtistsFixtures(t *testing.T) {
	data := t.TempDir()
	lib := filepath.Join(data, "music")
	if err := copyTree(filepath.Join("testdata", "library"), lib); err != nil {
		t.Fatal(err)
	}

	m := internal.NewModule(internal.Config{
		DataDir:    data,
		LibraryDir: lib,
		GRPCAddr:   "127.0.0.1:0",
		HTTPAddr:   "127.0.0.1:0",
	})
	if err := m.Init(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ScanLibrary(); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Stop(t.Context()) })

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(m.GRPCListenAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	mesh := meshv1.NewModuleMeshClient(conn)
	resp, err := mesh.Call(ctx, &meshv1.CallRequest{
		TargetModule: "media-music",
		Method:       "ListArtists",
		Payload:      []byte(`{"query":""}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetError() != "" {
		t.Fatalf("mesh error: %s", resp.GetError())
	}
	var artists []map[string]any
	if err := json.Unmarshal(resp.GetPayload(), &artists); err != nil {
		t.Fatal(err)
	}
	if len(artists) < 1 {
		t.Fatal("expected fixture artists via mesh")
	}

	hr, err := http.Get("http://" + m.HTTPListenAddr() + "/api/artists")
	if err != nil {
		t.Fatal(err)
	}
	defer hr.Body.Close()
	if hr.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(hr.Body)
		t.Fatalf("status %d: %s", hr.StatusCode, b)
	}
	body, err := io.ReadAll(hr.Body)
	if err != nil {
		t.Fatal(err)
	}
	artists = nil
	if err := json.Unmarshal(body, &artists); err != nil {
		t.Fatal(err)
	}
	if len(artists) < 1 {
		t.Fatal("expected fixture artists via HTTP API")
	}

	id, _ := artists[0]["id"].(string)
	if id == "" {
		t.Fatal("expected artist id")
	}
	detailResp, err := mesh.Call(ctx, &meshv1.CallRequest{
		TargetModule: "media-music",
		Method:       "GetArtist",
		Payload:      []byte(`{"id":"` + id + `"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if detailResp.GetError() != "" {
		t.Fatalf("GetArtist mesh error: %s", detailResp.GetError())
	}
}
