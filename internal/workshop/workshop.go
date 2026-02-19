package workshop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
)

type ModMetadata struct {
	ID        string
	Title     string
	UpdatedAt time.Time
}

type Client interface {
	FetchMetadata(ctx context.Context, modIDs []string) (map[string]ModMetadata, error)
}

type WebAPIClient struct {
	httpClient *http.Client
	apiKey     string
	endpoint   string
	maxRetries int
	backoff    time.Duration
}

func NewWebAPIClient(apiKey string, timeout time.Duration, maxRetries int, backoff time.Duration) *WebAPIClient {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if backoff <= 0 {
		backoff = 500 * time.Millisecond
	}
	return &WebAPIClient{
		httpClient: &http.Client{Timeout: timeout},
		apiKey:     apiKey,
		endpoint:   "https://api.steampowered.com/ISteamRemoteStorage/GetPublishedFileDetails/v1/",
		maxRetries: maxRetries,
		backoff:    backoff,
	}
}

func (c *WebAPIClient) FetchMetadata(ctx context.Context, modIDs []string) (map[string]ModMetadata, error) {
	if len(modIDs) == 0 {
		return map[string]ModMetadata{}, nil
	}

	vals := url.Values{}
	if c.apiKey != "" {
		vals.Set("key", c.apiKey)
	}
	vals.Set("itemcount", strconv.Itoa(len(modIDs)))
	for i, id := range modIDs {
		vals.Set(fmt.Sprintf("publishedfileids[%d]", i), id)
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewBufferString(vals.Encode()))
		if err != nil {
			return nil, fmt.Errorf("create workshop request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request workshop metadata: %w", err)
		} else {
			if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("workshop api returned status %d", resp.StatusCode)
				resp.Body.Close()
			} else if resp.StatusCode >= 300 {
				err := fmt.Errorf("workshop api returned status %d", resp.StatusCode)
				resp.Body.Close()
				return nil, err
			} else {
				meta, err := parseMetadataResponse(resp)
				resp.Body.Close()
				if err != nil {
					return nil, err
				}
				return meta, nil
			}
		}

		if attempt == c.maxRetries || errors.Is(lastErr, context.Canceled) || errors.Is(lastErr, context.DeadlineExceeded) {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.backoff * time.Duration(attempt)):
		}
	}

	return nil, lastErr
}

func PollMetadata(ctx context.Context, cfg config.Config, st *state.State, client Client, now time.Time) ([]string, error) {
	candidateSet := make(map[string]struct{})
	for _, srv := range st.Servers {
		for _, id := range srv.LastModIDs {
			if id != "" {
				candidateSet[id] = struct{}{}
			}
		}
	}
	candidateModIDs := mapKeys(candidateSet)

	pollEvery := time.Duration(cfg.Intervals.WorkshopPollSeconds) * time.Second
	idsToCheck := make([]string, 0, len(candidateModIDs))
	for _, id := range candidateModIDs {
		mod := st.Mods[id]
		if !mod.LastWorkshopCheckAt.IsZero() && now.Sub(mod.LastWorkshopCheckAt) < pollEvery {
			continue
		}
		idsToCheck = append(idsToCheck, id)
	}

	results, err := fetchBatched(ctx, client, idsToCheck, cfg.Concurrency.WorkshopBatchSize, cfg.Concurrency.WorkshopParallelism)
	if err != nil {
		return nil, err
	}

	for _, id := range idsToCheck {
		mod := st.Mods[id]
		mod.LastWorkshopCheckAt = now.UTC()
		if meta, ok := results[id]; ok {
			if meta.Title != "" {
				mod.LastTitle = meta.Title
			}
			if meta.UpdatedAt.After(mod.WorkshopUpdatedAt) {
				mod.WorkshopUpdatedAt = meta.UpdatedAt.UTC()
			}
		}
		st.Mods[id] = mod
	}

	modsToUpdateLocally := make([]string, 0)
	for _, id := range candidateModIDs {
		if needsLocalUpdate(st.Mods[id]) {
			modsToUpdateLocally = append(modsToUpdateLocally, id)
		}
	}
	sort.Strings(modsToUpdateLocally)
	return modsToUpdateLocally, nil
}

func fetchBatched(ctx context.Context, client Client, ids []string, batchSize int, parallelism int) (map[string]ModMetadata, error) {
	if len(ids) == 0 {
		return map[string]ModMetadata{}, nil
	}
	if batchSize <= 0 {
		batchSize = len(ids)
	}
	if parallelism <= 0 {
		parallelism = 1
	}

	batches := make([][]string, 0, (len(ids)+batchSize-1)/batchSize)
	for i := 0; i < len(ids); i += batchSize {
		end := i + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batches = append(batches, ids[i:end])
	}

	out := make(map[string]ModMetadata, len(ids))
	var mu sync.Mutex
	sem := make(chan struct{}, parallelism)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	for _, batch := range batches {
		wg.Add(1)
		batch := append([]string(nil), batch...)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			meta, err := client.FetchMetadata(ctx, batch)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			mu.Lock()
			for id, d := range meta {
				out[id] = d
			}
			mu.Unlock()
		}()
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func parseMetadataResponse(resp *http.Response) (map[string]ModMetadata, error) {
	var payload struct {
		Response struct {
			PublishedFileDetails []struct {
				PublishedFileID string `json:"publishedfileid"`
				Title           string `json:"title"`
				TimeUpdated     int64  `json:"time_updated"`
			} `json:"publishedfiledetails"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode workshop response: %w", err)
	}

	mods := make(map[string]ModMetadata, len(payload.Response.PublishedFileDetails))
	for _, detail := range payload.Response.PublishedFileDetails {
		mods[detail.PublishedFileID] = ModMetadata{
			ID:        detail.PublishedFileID,
			Title:     detail.Title,
			UpdatedAt: time.Unix(detail.TimeUpdated, 0).UTC(),
		}
	}
	return mods, nil
}

func mapKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func needsLocalUpdate(mod state.ModState) bool {
	return mod.LocalUpdatedAt.IsZero() || mod.WorkshopUpdatedAt.After(mod.LocalUpdatedAt)
}

type NoopClient struct{}

func (c *NoopClient) FetchMetadata(ctx context.Context, modIDs []string) (map[string]ModMetadata, error) {
	_ = ctx
	_ = modIDs
	return map[string]ModMetadata{}, nil
}
