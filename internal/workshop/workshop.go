package workshop

import "context"

type ModMetadata struct {
	ID           string
	Title        string
	UpdatedEpoch int64
}

type Client interface {
	FetchMetadata(ctx context.Context, modIDs []string) (map[string]ModMetadata, error)
}

type NoopClient struct{}

func (c *NoopClient) FetchMetadata(ctx context.Context, modIDs []string) (map[string]ModMetadata, error) {
	// TODO: implement Steam Workshop API integration.
	_ = ctx
	return map[string]ModMetadata{}, nil
}
