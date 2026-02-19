package localmods

import "github.com/example/dayz-standalone-mode-updater/internal/config"

type Store interface {
	ResolvePath(mod config.ModConfig) (string, error)
}

type FilesystemStore struct{}

func (s *FilesystemStore) ResolvePath(mod config.ModConfig) (string, error) {
	// TODO: validate local filesystem paths and manifests.
	return mod.LocalPath, nil
}
