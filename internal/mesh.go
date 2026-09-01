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
	meshMethodScanLibrary = "ScanLibrary"
)

func meshResponseErr(msg string) (*meshv1.CallResponse, error) {
	return &meshv1.CallResponse{Error: msg}, nil
}

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
	Path          string `json:"path"`
	Monitored     bool   `json:"monitored"`
}

type albumJSON struct {
	ID            string      `json:"id"`
	ArtistID      string      `json:"artist_id"`
	Title         string      `json:"title"`
	MusicBrainzID string      `json:"musicbrainz_id"`
	Tracks        []trackJSON `json:"tracks,omitempty"`
	Year          int32       `json:"year"`
	Monitored     bool        `json:"monitored"`
}

type trackJSON struct {
	ID        string `json:"id"`
	AlbumID   string `json:"album_id"`
	ArtistID  string `json:"artist_id"`
	Title     string `json:"title"`
	Path      string `json:"path"`
	StreamURL string `json:"stream_url,omitempty"`
}

type artistDetailJSON struct {
	Artist artistJSON  `json:"artist"`
	Albums []albumJSON `json:"albums"`
}

// musicMeshServer extends the settings ModuleMesh with library list/detail methods
// so admin-ui can soft-proxy without a MusicManagementService client dependency.
type musicMeshServer struct {
	meshv1.UnimplementedModuleMeshServer
	settings modulesdk.SettingsHandler
	m        *Module
	moduleID string
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
		return meshResponseErr(fmt.Sprintf("wrong target module %q", req.GetTargetModule()))
	}
	switch req.GetMethod() {
	case "Settings":
		if s.settings.List == nil {
			return meshResponseErr("settings list not implemented")
		}
		defs := s.settings.List()
		raw, err := json.Marshal(defs)
		if err != nil {
			return meshResponseErr(err.Error())
		}
		return &meshv1.CallResponse{Payload: raw}, nil
	case "UpdateSetting":
		if s.settings.Update == nil {
			return meshResponseErr("settings update not implemented")
		}
		var body struct {
			Key   string `json:"Key"`
			Value string `json:"Value"`
		}
		if len(req.GetPayload()) > 0 {
			if err := json.Unmarshal(req.GetPayload(), &body); err != nil {
				return meshResponseErr(fmt.Sprintf("invalid UpdateSetting payload: %v", err))
			}
		}
		if body.Key == "" {
			return meshResponseErr("UpdateSetting requires Key")
		}
		if err := s.settings.Update(body.Key, body.Value); err != nil {
			return meshResponseErr(err.Error())
		}
		return &meshv1.CallResponse{Payload: []byte(`{"ok":true}`)}, nil
	case meshMethodListArtists:
		return s.listArtists(ctx, req.GetPayload())
	case meshMethodGetArtist:
		return s.getArtist(ctx, req.GetPayload())
	case meshMethodScanLibrary:
		return s.scanLibrary(ctx)
	default:
		return meshResponseErr(fmt.Sprintf("unknown method %q", req.GetMethod()))
	}
}

func (s *musicMeshServer) StreamCall(stream meshv1.ModuleMesh_StreamCallServer) error {
	return fmt.Errorf("StreamCall not supported for music mesh handler")
}

func (s *musicMeshServer) listArtists(ctx context.Context, payload []byte) (*meshv1.CallResponse, error) {
	var body listArtistsReq
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &body); err != nil {
			return meshResponseErr(fmt.Sprintf("invalid ListArtists payload: %v", err))
		}
	}
	if s.m.store == nil {
		return meshResponseErr("store not open")
	}
	items, err := s.m.store.ListArtists(ctx, body.Query)
	if err != nil {
		return meshResponseErr(err.Error())
	}
	out := make([]artistJSON, 0, len(items))
	for _, a := range items {
		out = append(out, toArtistJSON(a))
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return meshResponseErr(err.Error())
	}
	return &meshv1.CallResponse{Payload: raw}, nil
}

func (s *musicMeshServer) getArtist(ctx context.Context, payload []byte) (*meshv1.CallResponse, error) {
	var body getArtistReq
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &body); err != nil {
			return meshResponseErr(fmt.Sprintf("invalid GetArtist payload: %v", err))
		}
	}
	if body.ID == "" {
		return meshResponseErr("GetArtist requires id")
	}
	if s.m.store == nil {
		return meshResponseErr("store not open")
	}
	a, err := s.m.store.GetArtist(ctx, body.ID)
	if err != nil {
		return meshResponseErr(err.Error())
	}
	albums, err := s.m.store.ListAlbums(ctx, body.ID)
	if err != nil {
		return meshResponseErr(err.Error())
	}
	detail := artistDetailJSON{Artist: toArtistJSON(a), Albums: make([]albumJSON, 0, len(albums))}
	for _, al := range albums {
		detail.Albums = append(detail.Albums, s.m.albumJSONWithTracks(ctx, al))
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return meshResponseErr(err.Error())
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

func toTrackJSON(t *Track) trackJSON {
	return trackJSON{
		ID: t.ID, AlbumID: t.AlbumID, ArtistID: t.ArtistID, Title: t.Title, Path: t.Path,
		StreamURL: "/stream/music/" + t.ID,
	}
}

func (s *musicMeshServer) scanLibrary(ctx context.Context) (*meshv1.CallResponse, error) {
	res, err := s.m.ScanLibrary(ctx)
	if err != nil {
		return meshResponseErr(err.Error())
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return meshResponseErr(err.Error())
	}
	return &meshv1.CallResponse{Payload: raw}, nil
}

func (m *Module) albumJSONWithTracks(ctx context.Context, al *Album) albumJSON {
	out := toAlbumJSON(al)
	if m.store == nil {
		return out
	}
	tracks, err := m.store.ListTracks(ctx, al.ID, "")
	if err != nil {
		return out
	}
	out.Tracks = make([]trackJSON, 0, len(tracks))
	for _, t := range tracks {
		out.Tracks = append(out.Tracks, toTrackJSON(t))
	}
	return out
}
