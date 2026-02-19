package workshop

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/example/dayz-standalone-mode-updater/internal/config"
	"github.com/example/dayz-standalone-mode-updater/internal/state"
)

func TestParseMetadataResponseTimeUpdated(t *testing.T) {
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(`{"response":{"publishedfiledetails":[{"publishedfileid":"42","title":"Mod 42","time_updated":1700000000}]}}`))}
	got, err := parseMetadataResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("unexpected size: %d", len(got))
	}
	meta := got["42"]
	if !meta.UpdatedAt.Equal(time.Unix(1700000000, 0).UTC()) {
		t.Fatalf("unexpected timestamp: %s", meta.UpdatedAt)
	}
}

func TestPollMetadataBatchingAndCache(t *testing.T) {
	now := time.Unix(1700000100, 0).UTC()
	cfg := config.Config{
		Intervals: config.IntervalsConfig{WorkshopPollSeconds: 300},
		Concurrency: config.ConcurrencyConfig{
			WorkshopBatchSize:   2,
			WorkshopParallelism: 2,
		},
	}
	st := state.State{
		Mods: map[string]state.ModState{
			"1": {},
			"2": {LastWorkshopCheckAt: now.Add(-10 * time.Second)},
			"3": {LastWorkshopCheckAt: now.Add(-10 * time.Minute)},
		},
		Servers: map[string]state.ServerState{
			"a": {LastModIDs: []string{"1", "2", "3"}},
			"b": {LastModIDs: []string{"3"}},
		},
	}
	fc := &fakeClient{response: map[string]ModMetadata{
		"1": {ID: "1", UpdatedAt: now.Add(-time.Minute)},
		"3": {ID: "3", UpdatedAt: now.Add(-time.Minute)},
	}}

	_, err := PollMetadata(context.Background(), cfg, &st, fc, now)
	if err != nil {
		t.Fatal(err)
	}
	if fc.callCount() != 1 {
		t.Fatalf("expected one batch call, got %d", fc.callCount())
	}
	gotBatch := append([]string(nil), fc.calls[0]...)
	sort.Strings(gotBatch)
	if !reflect.DeepEqual(gotBatch, []string{"1", "3"}) {
		t.Fatalf("unexpected batch ids: %#v", gotBatch)
	}
	if st.Mods["2"].LastWorkshopCheckAt != now.Add(-10*time.Second) {
		t.Fatalf("mod 2 should not have been rechecked")
	}
	if !st.Mods["1"].LastWorkshopCheckAt.Equal(now) || !st.Mods["3"].LastWorkshopCheckAt.Equal(now) {
		t.Fatalf("expected checked mods to be stamped with current time")
	}
}

func TestNeedsLocalUpdateDecision(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	cfg := config.Config{
		Intervals:   config.IntervalsConfig{WorkshopPollSeconds: 1},
		Concurrency: config.ConcurrencyConfig{WorkshopBatchSize: 10, WorkshopParallelism: 1},
	}
	st := state.State{
		Mods: map[string]state.ModState{
			"1": {LocalUpdatedAt: now.Add(-2 * time.Hour)},
			"2": {LocalUpdatedAt: now.Add(-time.Hour)},
			"3": {},
		},
		Servers: map[string]state.ServerState{
			"a": {LastModIDs: []string{"1", "2", "3"}},
		},
	}
	fc := &fakeClient{response: map[string]ModMetadata{
		"1": {ID: "1", UpdatedAt: now.Add(-time.Hour)},
		"2": {ID: "2", UpdatedAt: now.Add(-2 * time.Hour)},
		"3": {ID: "3", UpdatedAt: now.Add(-30 * time.Minute)},
	}}
	got, err := PollMetadata(context.Background(), cfg, &st, fc, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"1", "3"}) {
		t.Fatalf("unexpected mods needing local update: %#v", got)
	}
}

type fakeClient struct {
	mu       sync.Mutex
	calls    [][]string
	response map[string]ModMetadata
}

func (f *fakeClient) FetchMetadata(_ context.Context, modIDs []string) (map[string]ModMetadata, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]string(nil), modIDs...))
	f.mu.Unlock()
	out := make(map[string]ModMetadata)
	for _, id := range modIDs {
		if m, ok := f.response[id]; ok {
			out[id] = m
		}
	}
	return out, nil
}

func (f *fakeClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}
