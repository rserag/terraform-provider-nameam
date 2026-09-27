package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGetDomainsFollowsPagination(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/client/domains" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer jwt" {
			t.Fatalf("authorization = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Fatalf("limit = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("page") {
		case "1":
			_, _ = w.Write([]byte(`{"docs":[{"_id":"domain-1","domain":"one.am"}],"totalDocs":2,"limit":50,"page":1,"totalPages":2,"hasNextPage":true,"nextPage":2}`))
		case "2":
			_, _ = w.Write([]byte(`{"docs":[{"_id":"domain-2","domain":"two.am"}],"totalDocs":2,"limit":50,"page":2,"totalPages":2,"hasNextPage":false,"nextPage":null}`))
		default:
			t.Fatalf("unexpected page: %s", r.URL.Query().Get("page"))
		}
	}))
	defer server.Close()

	c := testClient(t, server.URL, 0)
	response, err := c.GetDomains(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(response.Docs) != 2 {
		t.Fatalf("calls=%d docs=%+v", calls, response.Docs)
	}
	if response.Docs[0].ID != "domain-1" || response.Docs[1].Domain != "two.am" {
		t.Fatalf("unexpected domains: %+v", response.Docs)
	}
}

func TestGetDomainsRejectsPaginationLoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"docs":[],"page":1,"hasNextPage":true,"nextPage":1}`))
	}))
	defer server.Close()

	_, err := testClient(t, server.URL, 0).GetDomains(context.Background())
	if err == nil {
		t.Fatal("expected pagination loop error")
	}
}

func TestGetRetriesCanBeDisabled(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"temporary"}`))
	}))
	defer server.Close()

	_, err := testClient(t, server.URL, 0).GetDomains(context.Background())
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestUpdateDomainDeleteByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/client/domains/agtc.am" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var request UpdateDomainRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if len(request.Records) != 1 || request.Records[0].ID != "abc" || request.Records[0].Action != "DELETE" {
			t.Fatalf("request = %+v", request)
		}
		_, _ = w.Write([]byte(`{"_id":"domain-1","domain":"agtc.am","records":[]}`))
	}))
	defer server.Close()

	_, err := testClient(t, server.URL, 0).UpdateDomain(context.Background(), "agtc.am", UpdateDomainRequest{
		Records: []UpdateRecord{{ID: "abc", Action: "DELETE"}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateDomainReconcilesAppliedMutation(t *testing.T) {
	var putCalls int
	created := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCalls++
			created = true
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"response lost"}`))
			return
		}
		writeDomainPage(w, createdRecords(created))
	}))
	defer server.Close()

	domain, err := testClient(t, server.URL, 2).UpdateDomain(context.Background(), "example.am", createRequest())
	if err != nil {
		t.Fatal(err)
	}
	if putCalls != 1 || len(domain.Records) != 1 {
		t.Fatalf("putCalls=%d domain=%+v", putCalls, domain)
	}
}

func TestUpdateDomainRetriesOnlyWhenMutationNotApplied(t *testing.T) {
	var putCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeDomainPage(w, nil)
			return
		}
		putCalls++
		if putCalls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"not applied"}`))
			return
		}
		_, _ = w.Write([]byte(`{"_id":"domain-1","domain":"example.am","records":[{"id":"rec-1","type":"A","ttl":300,"name":"www.example.am","content":"192.0.2.1"}]}`))
	}))
	defer server.Close()

	domain, err := testClient(t, server.URL, 1).UpdateDomain(context.Background(), "example.am", createRequest())
	if err != nil {
		t.Fatal(err)
	}
	if putCalls != 2 || len(domain.Records) != 1 {
		t.Fatalf("putCalls=%d domain=%+v", putCalls, domain)
	}
}

func TestUpdateDomainStopsOnPartialMutation(t *testing.T) {
	var putCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCalls++
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"partial"}`))
			return
		}
		writeDomainPage(w, nil)
	}))
	defer server.Close()

	request := UpdateDomainRequest{Records: []UpdateRecord{
		{ID: "old", Action: "DELETE"},
		{Type: "A", TTL: 300, Prefix: "www", Name: "www.example.am", Content: "192.0.2.1", Action: "CREATE"},
	}}
	_, err := testClient(t, server.URL, 2).UpdateDomain(context.Background(), "example.am", request)
	var partial *PartialMutationError
	if !errors.As(err, &partial) || putCalls != 1 {
		t.Fatalf("err=%v putCalls=%d", err, putCalls)
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	err := &HTTPError{StatusCode: http.StatusTooManyRequests, RetryAfter: 3 * time.Second}
	if got := retryDelay(err, time.Millisecond, 5); got != 3*time.Second {
		t.Fatalf("retry delay = %s", got)
	}
}

func TestGetDomainsReturnsStructuredHTTPErrorWithoutRetryingClientErrors(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	}))
	defer server.Close()

	_, err := testClient(t, server.URL, 3).GetDomains(context.Background())
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusUnauthorized || httpErr.Message != "invalid token" || calls != 1 {
		t.Fatalf("error=%v calls=%d", err, calls)
	}
}

func TestGetDomainsReportsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not-json-jwt`))
	}))
	defer server.Close()

	_, err := testClient(t, server.URL, 0).GetDomains(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode response") || strings.Contains(err.Error(), "jwt") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error=%v", err)
	}
}

func TestNewRejectsInvalidOptions(t *testing.T) {
	t.Parallel()
	for name, options := range map[string]Options{
		"missing token":  {BaseURL: "https://api.name.am"},
		"relative URL":   {BaseURL: "/api", Token: "token"},
		"wrong scheme":   {BaseURL: "ftp://api.name.am", Token: "token"},
		"negative retry": {BaseURL: "https://api.name.am", Token: "token", Retries: -1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(options); err == nil {
				t.Fatalf("expected error for %+v", options)
			}
		})
	}
}

func testClient(t *testing.T, baseURL string, retries int) *Client {
	t.Helper()
	client, err := New(Options{BaseURL: baseURL, Token: "jwt", Timeout: 5 * time.Second, Retries: retries, RetryBackoff: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func createRequest() UpdateDomainRequest {
	return UpdateDomainRequest{Records: []UpdateRecord{{
		Type: "A", TTL: 300, Prefix: "www", Name: "www.example.am", Content: "192.0.2.1", Action: "CREATE",
	}}}
}

func createdRecords(created bool) []DNSRecord {
	if !created {
		return nil
	}
	return []DNSRecord{{ID: "rec-1", Type: "A", TTL: 300, Name: "www.example.am", Content: "192.0.2.1"}}
}

func writeDomainPage(w http.ResponseWriter, records []DNSRecord) {
	response := DomainsResponse{
		Docs:      []Domain{{ID: "domain-1", Domain: "example.am", Records: records}},
		TotalDocs: 1, Limit: domainsPageSize, Page: 1, TotalPages: 1,
	}
	data, _ := json.Marshal(response)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}
