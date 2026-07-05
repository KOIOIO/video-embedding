package recommendation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type GorseClient interface {
	Recommend(ctx context.Context, userID uint64, n int) ([]uint64, error)
	PutFeedback(ctx context.Context, feedback []GorseFeedback) error
	UpsertUsers(ctx context.Context, users []GorseUser) error
	UpsertItems(ctx context.Context, items []GorseItem) error
	PatchItem(ctx context.Context, item GorseItem) error
}

type GorseClientConfig struct {
	Endpoint string
	APIKey   string
	Timeout  time.Duration
}

type GorseHTTPClient struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

func NewGorseHTTPClient(cfg GorseClientConfig) *GorseHTTPClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &GorseHTTPClient{
		endpoint: strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/"),
		apiKey:   strings.TrimSpace(cfg.APIKey),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *GorseHTTPClient) Timeout() time.Duration {
	if c == nil || c.httpClient == nil {
		return 0
	}
	return c.httpClient.Timeout
}

func (c *GorseHTTPClient) Recommend(ctx context.Context, userID uint64, n int) ([]uint64, error) {
	if userID == 0 {
		return nil, nil
	}
	if n <= 0 {
		n = 10
	}
	endpoint, err := c.url("/api/recommend/" + strconv.FormatUint(userID, 10))
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	q.Set("n", strconv.Itoa(n))
	endpoint.RawQuery = q.Encode()

	var raw []string
	if err := c.doJSON(ctx, http.MethodGet, endpoint.String(), nil, &raw); err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(raw))
	for _, itemID := range raw {
		id, err := strconv.ParseUint(strings.TrimSpace(itemID), 10, 64)
		if err != nil || id == 0 {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *GorseHTTPClient) PutFeedback(ctx context.Context, feedback []GorseFeedback) error {
	if len(feedback) == 0 {
		return nil
	}
	return c.postOrPut(ctx, http.MethodPut, "/api/feedback", feedback)
}

func (c *GorseHTTPClient) UpsertUsers(ctx context.Context, users []GorseUser) error {
	if len(users) == 0 {
		return nil
	}
	return c.postOrPut(ctx, http.MethodPost, "/api/users", users)
}

func (c *GorseHTTPClient) UpsertItems(ctx context.Context, items []GorseItem) error {
	if len(items) == 0 {
		return nil
	}
	return c.postOrPut(ctx, http.MethodPost, "/api/items", items)
}

func (c *GorseHTTPClient) PatchItem(ctx context.Context, item GorseItem) error {
	itemID := strings.TrimSpace(item.ItemID)
	if itemID == "" {
		return nil
	}
	return c.postOrPut(ctx, http.MethodPatch, "/api/item/"+url.PathEscape(itemID), item)
}

func (c *GorseHTTPClient) postOrPut(ctx context.Context, method string, path string, payload any) error {
	endpoint, err := c.url(path)
	if err != nil {
		return err
	}
	return c.doJSON(ctx, method, endpoint.String(), payload, nil)
}

func (c *GorseHTTPClient) url(path string) (*url.URL, error) {
	if strings.TrimSpace(c.endpoint) == "" {
		return nil, fmt.Errorf("gorse endpoint is empty")
	}
	base, err := url.Parse(c.endpoint)
	if err != nil {
		return nil, err
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	return base, nil
}

func (c *GorseHTTPClient) doJSON(ctx context.Context, method string, endpoint string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gorse %s %s returned status %d", method, endpoint, resp.StatusCode)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
