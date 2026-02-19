package steam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	httpClient *http.Client
	apiKey     string
}

type ModDetails struct {
	ID        string
	Title     string
	UpdatedAt time.Time
}

func NewClient(apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		apiKey:     apiKey,
	}
}

func (c *Client) FetchModDetails(ctx context.Context, ids []string) ([]ModDetails, error) {
	vals := url.Values{}
	vals.Set("key", c.apiKey)
	vals.Set("itemcount", strconv.Itoa(len(ids)))
	for i, id := range ids {
		vals.Set(fmt.Sprintf("publishedfileids[%d]", i), id)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.steampowered.com/ISteamRemoteStorage/GetPublishedFileDetails/v1/",
		bytes.NewBufferString(vals.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create steam request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request steam details: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("steam returned status %d", resp.StatusCode)
	}
	return parseDetails(resp)
}

func parseDetails(resp *http.Response) ([]ModDetails, error) {
	var body struct {
		Response struct {
			PublishedFileDetails []struct {
				PublishedFileID string `json:"publishedfileid"`
				Title           string `json:"title"`
				TimeUpdated     int64  `json:"time_updated"`
			} `json:"publishedfiledetails"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode steam response: %w", err)
	}
	mods := make([]ModDetails, 0, len(body.Response.PublishedFileDetails))
	for _, d := range body.Response.PublishedFileDetails {
		mods = append(mods, ModDetails{
			ID:        d.PublishedFileID,
			Title:     d.Title,
			UpdatedAt: time.Unix(d.TimeUpdated, 0).UTC(),
		})
	}
	return mods, nil
}
