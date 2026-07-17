package recommendation

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

type GorseDashboardPoint struct {
	Timestamp time.Time
	Value     float64
}

type GorseDashboardClient interface {
	Timeseries(ctx context.Context, metric string, begin, end time.Time) ([]GorseDashboardPoint, error)
	PositiveFeedbackTypes(ctx context.Context) ([]string, error)
}

type GorseDashboardClientConfig struct {
	Endpoint string
	Username string
	Password string
	Timeout  time.Duration
}

type GorseDashboardHTTPClient struct {
	endpoint   string
	username   string
	password   string
	httpClient *http.Client
	loginMu    sync.Mutex
}

func NewGorseDashboardHTTPClient(cfg GorseDashboardClientConfig) *GorseDashboardHTTPClient {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	jar, _ := cookiejar.New(nil)
	return &GorseDashboardHTTPClient{
		endpoint: strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/"),
		username: strings.TrimSpace(cfg.Username),
		password: cfg.Password,
		httpClient: &http.Client{
			Timeout: timeout,
			Jar:     jar,
		},
	}
}

func (c *GorseDashboardHTTPClient) Timeseries(ctx context.Context, metric string, begin, end time.Time) ([]GorseDashboardPoint, error) {
	metric = strings.TrimSpace(metric)
	if metric == "" {
		return nil, fmt.Errorf("gorse dashboard metric is empty")
	}
	endpoint, err := url.JoinPath(c.endpoint, "api", "dashboard", "timeseries", metric)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	query.Set("begin", begin.Format(time.RFC3339))
	query.Set("end", end.Format(time.RFC3339))
	parsed.RawQuery = query.Encode()

	var raw []gorseDashboardPoint
	if err := c.getJSON(ctx, parsed.String(), &raw); err != nil {
		return nil, err
	}
	points := make([]GorseDashboardPoint, 0, len(raw))
	for _, point := range raw {
		timestamp, err := time.Parse(time.RFC3339, point.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("decode gorse dashboard timestamp: %w", err)
		}
		if math.IsNaN(point.Value) || math.IsInf(point.Value, 0) {
			return nil, fmt.Errorf("decode gorse dashboard value: non-finite number")
		}
		points = append(points, GorseDashboardPoint{Timestamp: timestamp, Value: point.Value})
	}
	return points, nil
}

func (c *GorseDashboardHTTPClient) PositiveFeedbackTypes(ctx context.Context) ([]string, error) {
	endpoint, err := url.JoinPath(c.endpoint, "api", "dashboard", "config")
	if err != nil {
		return nil, err
	}
	var cfg gorseDashboardConfig
	if err := c.getJSON(ctx, endpoint, &cfg); err != nil {
		return nil, err
	}
	return cfg.Recommend.DataSource.PositiveFeedbackTypes, nil
}

func (c *GorseDashboardHTTPClient) getJSON(ctx context.Context, endpoint string, out any) error {
	status, err := c.getJSONOnce(ctx, endpoint, out)
	if err != nil {
		return err
	}
	if status == http.StatusUnauthorized {
		if err := c.login(ctx); err != nil {
			return err
		}
		status, err = c.getJSONOnce(ctx, endpoint, out)
		if err != nil {
			return err
		}
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("gorse dashboard GET %s returned status %d", endpoint, status)
	}
	return nil
}

func (c *GorseDashboardHTTPClient) getJSONOnce(ctx context.Context, endpoint string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return resp.StatusCode, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp.StatusCode, fmt.Errorf("decode gorse dashboard response: %w", err)
	}
	return resp.StatusCode, nil
}

func (c *GorseDashboardHTTPClient) login(ctx context.Context) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()

	if c.endpoint == "" || c.username == "" || c.password == "" {
		return fmt.Errorf("gorse dashboard credentials are not configured")
	}
	endpoint, err := url.JoinPath(c.endpoint, "login")
	if err != nil {
		return err
	}
	form := url.Values{
		"user_name": {c.username},
		"password":  {c.password},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("gorse dashboard login returned status %d", resp.StatusCode)
	}
	return nil
}

type gorseDashboardPoint struct {
	Timestamp string  `json:"Timestamp"`
	Value     float64 `json:"Value"`
}

type gorseDashboardConfig struct {
	Recommend struct {
		DataSource struct {
			PositiveFeedbackTypes []string `json:"positive_feedback_types"`
		} `json:"data_source"`
	} `json:"recommend"`
}
