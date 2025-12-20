package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetDomains_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/client/domains" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Fatalf("missing bearer auth: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"docs":[{"domain":"agtc.am","dnsUsed":true,"nameServers":[{"hostname":"ns1"}],"records":[{"id":"abc","access":true,"type":"A","ttl":1,"name":"agtc.am","content":"1.2.3.4"}]}],"total":1,"limit":50,"page":1,"pages":1}`))
	}))
	defer srv.Close()

	c, err := New(Options{
		BaseURL: srv.URL,
		Token:   "jwt",
		Timeout: 5 * time.Second,
		Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := c.GetDomains(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Docs) != 1 || resp.Docs[0].Domain != "agtc.am" {
		t.Fatalf("unexpected docs: %+v", resp.Docs)
	}
	if len(resp.Docs[0].Records) != 1 || resp.Docs[0].Records[0].ID != "abc" {
		t.Fatalf("unexpected records: %+v", resp.Docs[0].Records)
	}
}

func TestUpdateDomain_DeleteByID_Success(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/client/domains/agtc.am" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"domain":"agtc.am","records":[]}`))
	}))
	defer srv.Close()

	c, err := New(Options{
		BaseURL: srv.URL,
		Token:   "jwt",
		Timeout: 5 * time.Second,
		Retries: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.UpdateDomain(context.Background(), "agtc.am", UpdateDomainRequest{
		Records: []UpdateRecord{{ID: "abc", Action: "DELETE"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, `"id":"abc"`) || !strings.Contains(gotBody, `"action":"DELETE"`) {
		t.Fatalf("unexpected body: %s", gotBody)
	}
}
