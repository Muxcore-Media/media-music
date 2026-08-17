package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc"

	meshv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/mesh/v1"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
)

const (
	meshMethodListArtists = "ListArtists"
	meshMethodGetArtist   = "GetArtist"
)

type listArtistsReq struct {
	Query string `json:"query"`
}

type getArtistReq struct {
	ID string `json:"id"`
}

type artistJSON struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	MusicBrainzID string `json:"musicbrainz_id"`
	Monitored     bool   `json:"monitored"`
	Path          string `json:"path"`
}

type albumJSON struct {
	ID            string `json:"id"`
	ArtistID      string `json:"artist_id"`
	Title         string `json:"title"`
	MusicBrainzID string `json:"musicbrainz_id"`
	Year          int32  `json:"year"`
	Monitored     bool   `json:"monitored"`
}

type artistDetailJSON struct {
	Artist artistJSON  `json:"artist"`
	Albums []albumJSON `json:"albums"`
}

// musicMeshServer extends the settings ModuleMesh with library list/detail methods
// so admin-ui can soft-proxy without a MusicManagementService client dependency.
type musicMeshServer struct {
	meshv1.UnimplementedModuleMeshServer
	moduleID string
	settings modulesdk.SettingsHandler
	m        *Module
}

func registerMusicMesh(srv *grpc.Server, moduleID string, m *Module) {
	meshv1.RegisterModuleMeshServer(srv, &musicMeshServer{
		moduleID: moduleID,
		settings: modulesdk.SettingsHandlerFromProvider(m),
		m:        m,
	})
}

func (s *musicMeshServer) Call(ctx context.Context, req *meshv1.CallRequest) (*meshv1.CallResponse, error) {
	if req.GetTargetModule() != "" && req.GetTargetModule() != s.moduleID {
		return &meshv1.CallResponse{Error: fmt.Sprintf("wrong target module %q", req.GetTargetModule())}, nil
	}
	switch req.GetMethod() {
	case "Settings":
		if s.settings.List == nil {
			return &meshv1.CallResponse{Error: "settings list not implemented"}, nil
		}
		defs := s.settings.List()
		raw, err := json.Marshal(defs)
		if err != nil {
			return &meshv1.CallResponse{Error: err.Error()}, nil
		}
		return &meshv1.CallResponse{Payload: raw}, nil
	case "UpdateSetting":
		if s.settings.Update == nil {
			return &meshv1.CallResponse{Error: "settings update not implemented"}, nil
		}
		var body struct {
			Key   string `json:"Key"`
			Value string `json:"Value"`
		}
		if len(req.GetPayload()) > 0 {
			if err := json.Unmarshal(req.GetPayload(), &body); err != nil {
				return &meshv1.CallResponse{Error: fmt.Sprintf("invalid UpdateSetting payload: %v", err)}, nil
			}
		}
		if body.Key == "" {
			return &meshv1.CallResponse{Error: "UpdateSetting requires Key"}, nil
		}
		if err := s.settings.Update(body.Key, body.Value); err != nil {
			return &meshv1.CallResponse{Error: err.Error()}, nil
		}
		return &meshv1.CallResponse{Payload: []byte(`{"ok":true}`)}, nil
	case meshMethodListArtists:
		return s.listArtists(req.GetPayload())
	case meshMethodGetArtist:
		return s.getArtist(req.GetPayload())
	default:
		return &meshv1.CallResponse{Error: fmt.Sprintf("unknown method %q", req.GetMethod())}, nil
	}
}

func (s *musicMeshServer) StreamCall(stream meshv1.ModuleMesh_StreamCallServer) error {
	return fmt.Errorf("StreamCall not supported for music mesh handler")
}

func (s *musicMeshServer) listArtists(payload []byte) (*meshv1.CallResponse, error) {
	var body listArtistsReq
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &body); err != nil {
			return &meshv1.CallResponse{Error: fmt.Sprintf("invalid ListArtists payload: %v", err)}, nil
		}
	}
	if s.m.store == nil {
		return &meshv1.CallResponse{Error: "store not open"}, nil
	}
	items, err := s.m.store.ListArtists(body.Query)
	if err != nil {
		return &meshv1.CallResponse{Error: err.Error()}, nil
	}
	out := make([]artistJSON, 0, len(items))
	for _, a := range items {
		out = append(out, toArtistJSON(a))
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return &meshv1.CallResponse{Error: err.Error()}, nil
	}
	return &meshv1.CallResponse{Payload: raw}, nil
}

func (s *musicMeshServer) getArtist(payload []byte) (*meshv1.CallResponse, error) {
	var body getArtistReq
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &body); err != nil {
			return &meshv1.CallResponse{Error: fmt.Sprintf("invalid GetArtist payload: %v", err)}, nil
		}
	}
	if body.ID == "" {
		return &meshv1.CallResponse{Error: "GetArtist requires id"}, nil
	}
	if s.m.store == nil {
		return &meshv1.CallResponse{Error: "store not open"}, nil
	}
	a, err := s.m.store.GetArtist(body.ID)
	if err != nil {
		return &meshv1.CallResponse{Error: err.Error()}, nil
	}
	albums, err := s.m.store.ListAlbums(body.ID)
	if err != nil {
		return &meshv1.CallResponse{Error: err.Error()}, nil
	}
	detail := artistDetailJSON{Artist: toArtistJSON(a), Albums: make([]albumJSON, 0, len(albums))}
	for _, al := range albums {
		detail.Albums = append(detail.Albums, toAlbumJSON(al))
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return &meshv1.CallResponse{Error: err.Error()}, nil
	}
	return &meshv1.CallResponse{Payload: raw}, nil
}

func toArtistJSON(a *Artist) artistJSON {
	return artistJSON{
		ID: a.ID, Name: a.Name, MusicBrainzID: a.MusicBrainzID,
		Monitored: a.Monitored, Path: a.Path,
	}
}

func toAlbumJSON(a *Album) albumJSON {
	return albumJSON{
		ID: a.ID, ArtistID: a.ArtistID, Title: a.Title,
		MusicBrainzID: a.MusicBrainzID, Year: a.Year, Monitored: a.Monitored,
	}
}
