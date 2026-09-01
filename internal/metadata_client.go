package internal

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"

	musicbrainzv1 "github.com/Muxcore-Media/metadata-musicbrainz/proto/musicbrainzv1"
)

const capMetadataMusic = "metadata.music"

type musicBrainzAPI interface {
	SearchArtist(ctx context.Context, in *musicbrainzv1.SearchArtistRequest, opts ...grpc.CallOption) (*musicbrainzv1.SearchArtistResponse, error)
	GetArtist(ctx context.Context, in *musicbrainzv1.GetArtistRequest, opts ...grpc.CallOption) (*musicbrainzv1.GetArtistResponse, error)
}

func (m *Module) withMusicBrainz(ctx context.Context) (musicBrainzAPI, func(), error) {
	if m.musicBrainzOverride != nil {
		return m.musicBrainzOverride, func() {}, nil
	}
	if m.mc == nil {
		return nil, nil, fmt.Errorf("not connected to core")
	}
	mods, err := m.mc.Discovery.FindByCapability(ctx, capMetadataMusic)
	if err != nil {
		return nil, nil, err
	}
	if len(mods) == 0 {
		mods, err = m.mc.Discovery.FindByCapability(ctx, "metadata.musicbrainz")
		if err != nil {
			return nil, nil, err
		}
	}
	if len(mods) == 0 {
		return nil, nil, fmt.Errorf("no music metadata module found")
	}
	mod := mods[0]
	addr := dialAddrForModule(mod.Id, mod.GetHttpAddr())
	if addr == "" {
		return nil, nil, fmt.Errorf("music metadata module has no address")
	}
	conn, err := grpc.NewClient(addr, meshGRPCDialOpts()...)
	if err != nil {
		return nil, nil, fmt.Errorf("dial metadata-musicbrainz: %w", err)
	}
	return musicbrainzv1.NewMusicBrainzServiceClient(conn), func() { _ = conn.Close() }, nil
}

func (m *Module) resolveArtistMBID(ctx context.Context, a *Artist) (string, error) {
	if a == nil {
		return "", nil
	}
	mbid := strings.TrimSpace(a.MusicBrainzID)
	if mbid != "" {
		return mbid, nil
	}
	name := strings.TrimSpace(a.Name)
	if name == "" {
		return "", nil
	}
	cli, closeFn, err := m.withMusicBrainz(ctx)
	if err != nil {
		return "", err
	}
	defer closeFn()

	resp, err := cli.SearchArtist(ctx, &musicbrainzv1.SearchArtistRequest{Query: name, Limit: 5})
	if err != nil {
		return "", err
	}
	mbid = pickMusicBrainzID(name, resp.GetArtists())
	if mbid == "" {
		return "", nil
	}
	if _, err := m.store.UpdateArtist(ctx, a.ID, map[string]any{"musicbrainz_id": mbid}); err != nil {
		return "", err
	}
	return mbid, nil
}

func pickMusicBrainzID(name string, results []*musicbrainzv1.ArtistSearchResult) string {
	want := strings.ToLower(strings.TrimSpace(name))
	var best *musicbrainzv1.ArtistSearchResult
	for _, r := range results {
		if r == nil || strings.TrimSpace(r.GetId()) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(r.GetName()), name) {
			return r.GetId()
		}
		if best == nil || r.GetScore() > best.GetScore() {
			best = r
		}
	}
	if best != nil && strings.ToLower(strings.TrimSpace(best.GetName())) == want {
		return best.GetId()
	}
	if best != nil && best.GetScore() >= 95 {
		return best.GetId()
	}
	return ""
}
