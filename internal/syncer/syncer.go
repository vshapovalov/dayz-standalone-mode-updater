package syncer

import "context"

type Syncer interface {
	SyncMod(ctx context.Context, localPath string, remoteDir string) error
}

type NoopSyncer struct{}

func (s *NoopSyncer) SyncMod(ctx context.Context, localPath string, remoteDir string) error {
	// TODO: implement transport-specific sync logic.
	_ = ctx
	_ = localPath
	_ = remoteDir
	return nil
}
