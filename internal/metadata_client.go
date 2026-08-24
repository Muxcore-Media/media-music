package internal

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	musicbrainzv1 "github.com/Muxcore-Media/metadata-musicbrainz/proto/musicbrainzv1"
)

const capMetadataMusic = "metadata.music"

func (m *Module) musicBrainzClient(ctx context.Context) (musicbrainzv1.MusicBrainzServiceClient, func(), error) {
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
	addr := mods[0].GetHttpAddr()
	if addr == "" {
		return nil, nil, fmt.Errorf("music metadata module has no address")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("dial metadata-musicbrainz: %w", err)
	}
	return musicbrainzv1.NewMusicBrainzServiceClient(conn), func() { _ = conn.Close() }, nil
}
