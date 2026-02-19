package steamcmd

import "context"

type Runner interface {
	UpdateMods(ctx context.Context, modIDs []string) error
}

type NoopRunner struct{}

func (r *NoopRunner) UpdateMods(ctx context.Context, modIDs []string) error {
	// TODO: invoke SteamCMD and parse output.
	_ = ctx
	_ = modIDs
	return nil
}
