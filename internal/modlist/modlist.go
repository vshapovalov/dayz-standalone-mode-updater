package modlist

import "github.com/example/dayz-standalone-mode-updater/internal/config"

type Provider interface {
	Load() ([]config.ModConfig, error)
}

type StaticProvider struct {
	mods []config.ModConfig
}

func NewStaticProvider(mods []config.ModConfig) *StaticProvider {
	return &StaticProvider{mods: mods}
}

func (p *StaticProvider) Load() ([]config.ModConfig, error) {
	// TODO: support additional mod list sources.
	return p.mods, nil
}
