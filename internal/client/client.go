package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"golang.org/x/time/rate"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client

	retries      int
	retryBackoff time.Duration
	rateLimiter  *rate.Limiter
}

type Options struct {
	BaseURL      string
	Token        string
	Timeout      time.Duration
	Retries      int
	RetryBackoff time.Duration
	RateLimit    float64
}

func New(opts Options) (*Client, error) {
	if strings.TrimSpace(opts.Token) == "" {
		return nil, fmt.Errorf("token is required")
	}
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = "https://api.name.am"
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	retries := opts.Retries
	if retries < 0 {
		retries = 0
	}
	if retries == 0 {
		retries = 3
	}

	backoff := opts.RetryBackoff
	if backoff <= 0 {
		backoff = 1 * time.Second
	}

	// Initialize rate limiter (default: 10 req/s, 0 = disabled)
	rateLimit := opts.RateLimit
	if rateLimit <= 0 {
		// If not set or 0, use default of 10 req/s
		rateLimit = 10.0
	}
	// rate.Limiter uses tokens per second, with burst of 1
	limiter := rate.NewLimiter(rate.Limit(rateLimit), 1)

	return &Client{
		baseURL: base,
		token:   opts.Token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retries:      retries,
		retryBackoff: backoff,
		rateLimiter:  limiter,
	}, nil
}

func (c *Client) GetDomains(ctx context.Context) (*DomainsResponse, error) {
	// TODO: pagination is present in response, but request query params are not documented.
	// Implement paging once Name.am documents it.
	var out DomainsResponse
	if err := c.doJSON(ctx, http.MethodGet, "/client/domains", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateDomain(ctx context.Context, domain string, req UpdateDomainRequest) (*Domain, error) {
	path := fmt.Sprintf("/client/domains/%s", domain)
	var out Domain
	if err := c.doJSON(ctx, http.MethodPut, path, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, in any, out any) error {
	// Apply rate limiting before making the request
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limiter wait: %w", err)
		}
	}

	url := c.baseURL + path

	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(b)
	}

	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return fmt.Errorf("new request: %w", err)
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.token)
		if in != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if isRetryableNetErr(err) && attempt < c.retries {
				lastErr = err
				sleepBackoff(ctx, c.retryBackoff, attempt)
				// rewind body for retry
				if in != nil {
					// recreate body reader
					b, _ := json.Marshal(in)
					body = bytes.NewReader(b)
				}
				continue
			}
			return fmt.Errorf("request failed: %w", err)
		}

		defer resp.Body.Close()

		rb, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			if out == nil {
				return nil
			}
			if len(rb) == 0 {
				return nil
			}
			if err := json.Unmarshal(rb, out); err != nil {
				return fmt.Errorf("decode response: %w (body=%s)", err, sanitizeBody(rb))
			}
			return nil
		}

		// Retry transient HTTP codes
		if (resp.StatusCode == 429 || resp.StatusCode >= 500) && attempt < c.retries {
			lastErr = fmt.Errorf("http %d: %s", resp.StatusCode, sanitizeBody(rb))
			sleepBackoff(ctx, c.retryBackoff, attempt)
			if in != nil {
				b, _ := json.Marshal(in)
				body = bytes.NewReader(b)
			}
			continue
		}

		return fmt.Errorf("http %d: %s", resp.StatusCode, sanitizeBody(rb))
	}

	return fmt.Errorf("request failed after retries: %w", lastErr)
}

func sanitizeBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 2000 {
		return s[:2000] + "...(truncated)"
	}
	return s
}

func sleepBackoff(ctx context.Context, base time.Duration, attempt int) {
	d := base
	for i := 0; i < attempt; i++ {
		d *= 2
		if d > 10*time.Second {
			d = 10 * time.Second
			break
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return
	case <-t.C:
		return
	}
}

func isRetryableNetErr(err error) bool {
	var ne net.Error
	if ok := (err != nil) && (strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "connection reset")); ok {
		return true
	}
	if err != nil && (errorsAs(err, &ne) && (ne.Timeout() || ne.Temporary())) {
		return true
	}
	return false
}

// local wrapper so we don't import errors in multiple places; keeps this file self-contained.
func errorsAs(err error, target any) bool {
	type aser interface{ As(any) bool }
	if e, ok := err.(aser); ok {
		return e.As(target)
	}
	return false
}
