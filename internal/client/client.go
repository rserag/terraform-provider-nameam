package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const domainsPageSize = 50

var ErrDomainNotFound = errors.New("Name.am domain not found")

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client

	retries      int
	retryBackoff time.Duration
}

type Options struct {
	BaseURL      string
	Token        string
	Timeout      time.Duration
	Retries      int
	RetryBackoff time.Duration
}

type HTTPError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.Message)
}

type PartialMutationError struct {
	Domain string
	Cause  error
}

func (e *PartialMutationError) Error() string {
	return fmt.Sprintf("DNS mutation for %s was only partially applied after an ambiguous response; inspect the zone before retrying: %v", e.Domain, e.Cause)
}

func (e *PartialMutationError) Unwrap() error { return e.Cause }

func New(opts Options) (*Client, error) {
	if strings.TrimSpace(opts.Token) == "" {
		return nil, fmt.Errorf("token is required")
	}
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = "https://api.name.am"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("base URL must be an absolute HTTP or HTTPS URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("base URL must use HTTP or HTTPS")
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if opts.Retries < 0 {
		return nil, fmt.Errorf("retries must be at least zero")
	}

	backoff := opts.RetryBackoff
	if backoff <= 0 {
		backoff = 1 * time.Second
	}

	return &Client{
		baseURL: base,
		token:   opts.Token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retries:      opts.Retries,
		retryBackoff: backoff,
	}, nil
}

func (c *Client) GetDomains(ctx context.Context) (*DomainsResponse, error) {
	result := &DomainsResponse{Docs: make([]Domain, 0)}
	page := 1
	seenPages := map[int]struct{}{}

	for {
		if _, seen := seenPages[page]; seen {
			return nil, fmt.Errorf("Name.am pagination repeated page %d", page)
		}
		seenPages[page] = struct{}{}

		query := url.Values{}
		query.Set("page", strconv.Itoa(page))
		query.Set("limit", strconv.Itoa(domainsPageSize))

		var current DomainsResponse
		if err := c.doJSON(ctx, http.MethodGet, "/client/domains?"+query.Encode(), nil, &current); err != nil {
			return nil, err
		}
		result.Docs = append(result.Docs, current.Docs...)
		result.TotalDocs = current.TotalDocs
		result.Limit = current.Limit
		result.Page = current.Page
		result.TotalPages = current.TotalPages
		result.HasPrevPage = current.HasPrevPage
		result.HasNextPage = current.HasNextPage
		result.PrevPage = current.PrevPage
		result.NextPage = current.NextPage

		if !current.HasNextPage {
			return result, nil
		}
		if current.NextPage != nil {
			page = *current.NextPage
		} else {
			page++
		}
	}
}

func (c *Client) GetDomain(ctx context.Context, domainName string) (*Domain, error) {
	response, err := c.GetDomains(ctx)
	if err != nil {
		return nil, err
	}
	wanted := normalizeDomain(domainName)
	for i := range response.Docs {
		if normalizeDomain(response.Docs[i].Domain) == wanted {
			return &response.Docs[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrDomainNotFound, wanted)
}

// UpdateDomain submits a DNS mutation. A PUT is never blindly retried: after
// an ambiguous transient failure, the client reads the zone and retries only
// when none of the requested actions took effect. A partially applied batch is
// returned as PartialMutationError for explicit operator recovery.
func (c *Client) UpdateDomain(ctx context.Context, domain string, request UpdateDomainRequest) (*Domain, error) {
	path := "/client/domains/" + url.PathEscape(domain)
	for attempt := 0; ; attempt++ {
		var result Domain
		err := c.doJSONOnce(ctx, http.MethodPut, path, request, &result)
		if err == nil {
			return &result, nil
		}
		if !isRetryableRequestError(err) {
			return nil, err
		}

		current, readErr := c.GetDomain(ctx, domain)
		if readErr != nil {
			return nil, fmt.Errorf("mutation failed and reconciliation read failed: %w (mutation error: %v)", readErr, err)
		}
		switch mutationOutcome(domain, current.Records, request.Records) {
		case mutationApplied:
			return current, nil
		case mutationPartial:
			return nil, &PartialMutationError{Domain: domain, Cause: err}
		case mutationNotApplied:
			if attempt >= c.retries {
				return nil, err
			}
			if waitErr := waitBackoff(ctx, retryDelay(err, c.retryBackoff, attempt)); waitErr != nil {
				return nil, waitErr
			}
		}
	}
}

func (c *Client) doJSON(ctx context.Context, method, path string, in, out any) error {
	for attempt := 0; ; attempt++ {
		err := c.doJSONOnce(ctx, method, path, in, out)
		if err == nil || method != http.MethodGet || !isRetryableRequestError(err) || attempt >= c.retries {
			return err
		}
		if waitErr := waitBackoff(ctx, retryDelay(err, c.retryBackoff, attempt)); waitErr != nil {
			return waitErr
		}
	}
}

func (c *Client) doJSONOnce(ctx context.Context, method, path string, in, out any) error {
	var payload []byte
	var err error
	if in != nil {
		payload, err = json.Marshal(in)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
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
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    c.responseMessage(responseBody, resp.StatusCode),
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	if out == nil || len(responseBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(responseBody, out); err != nil {
		return fmt.Errorf("decode response: %w (body=%s)", err, c.sanitizeBody(responseBody))
	}
	return nil
}

func (c *Client) responseMessage(body []byte, statusCode int) string {
	var response struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &response) == nil && response.Message != "" {
		return response.Message
	}
	if message := c.sanitizeBody(body); message != "" {
		return message
	}
	return http.StatusText(statusCode)
}

func (c *Client) sanitizeBody(body []byte) string {
	value := strings.TrimSpace(string(body))
	if c.token != "" {
		value = strings.ReplaceAll(value, c.token, "[REDACTED]")
	}
	if len(value) > 2000 {
		return value[:2000] + "...(truncated)"
	}
	return value
}

func isRetryableRequestError(err error) bool {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == http.StatusTooManyRequests || httpErr.StatusCode >= 500
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "connection reset") || strings.Contains(message, "unexpected eof")
}

func retryDelay(err error, base time.Duration, attempt int) time.Duration {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.RetryAfter > 0 {
		return httpErr.RetryAfter
	}
	delay := base
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= 10*time.Second {
			return 10 * time.Second
		}
	}
	return delay
}

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		if delay := time.Until(retryAt); delay > 0 {
			return delay
		}
	}
	return 0
}

func waitBackoff(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type mutationStatus int

const (
	mutationNotApplied mutationStatus = iota
	mutationApplied
	mutationPartial
)

func mutationOutcome(domain string, current []DNSRecord, changes []UpdateRecord) mutationStatus {
	applied := 0
	for _, change := range changes {
		switch strings.ToUpper(change.Action) {
		case "CREATE":
			if hasRecordByFields(domain, current, change) {
				applied++
			}
		case "DELETE":
			if !hasRecordByID(current, change.ID) {
				applied++
			}
		case "UPDATE":
			if hasRecordByIDAndFields(domain, current, change) {
				applied++
			}
		default:
			return mutationPartial
		}
	}
	if applied == 0 {
		return mutationNotApplied
	}
	if applied == len(changes) {
		return mutationApplied
	}
	return mutationPartial
}

func hasRecordByID(records []DNSRecord, id string) bool {
	for _, record := range records {
		if record.ID == id {
			return true
		}
	}
	return false
}

func hasRecordByFields(domain string, records []DNSRecord, wanted UpdateRecord) bool {
	for _, record := range records {
		if recordFieldsEqual(domain, record, wanted) {
			return true
		}
	}
	return false
}

func hasRecordByIDAndFields(domain string, records []DNSRecord, wanted UpdateRecord) bool {
	for _, record := range records {
		if record.ID == wanted.ID && recordFieldsEqual(domain, record, wanted) {
			return true
		}
	}
	return false
}

func recordFieldsEqual(domain string, actual DNSRecord, wanted UpdateRecord) bool {
	return strings.EqualFold(actual.Type, wanted.Type) &&
		normalizeRecordName(domain, actual.Name) == normalizeRecordName(domain, wanted.Name) &&
		actual.Content == wanted.Content && actual.TTL == wanted.TTL
}

func normalizeDomain(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

func normalizeRecordName(domain, value string) string {
	name := normalizeDomain(value)
	zone := normalizeDomain(domain)
	if name == "" || name == "@" || name == zone {
		return "@"
	}
	if strings.HasSuffix(name, "."+zone) {
		return strings.TrimSuffix(name, "."+zone)
	}
	return name
}
