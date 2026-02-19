package rcon

import "context"

type Notifier interface {
	BroadcastRestart(ctx context.Context, seconds int) error
}

type NoopNotifier struct{}

func (n *NoopNotifier) BroadcastRestart(ctx context.Context, seconds int) error {
	// TODO: send countdown messages and restart command.
	_ = ctx
	_ = seconds
	return nil
}
