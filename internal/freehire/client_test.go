package freehire

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSearchJobs(t *testing.T) {
	var capturedQuery string
	var capturedPath string

	client := NewClient("https://freehire.me/api/v1", "test-token")
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			capturedPath = r.URL.Path
			capturedQuery = r.URL.RawQuery
			body := `{
				"data": [
					{
						"public_slug": "go-engineer-acme-123",
						"title": "Senior Go Engineer",
						"company": "Acme",
						"location": "Prague",
						"work_mode": "hybrid",
						"url": "https://example.com/jobs/123",
						"description": "## Role\nBuild Go services.",
						"skills": ["go", "postgresql"],
						"enrichment": {
							"seniority": "senior",
							"category": "backend"
						}
					}
				],
				"meta": {"total": 1, "limit": 100, "offset": 0}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}

	jobs, err := client.SearchJobs(context.Background(), SearchParams{
		City:           "Prague",
		Categories:     []string{"backend", "software_engineering"},
		OpenWithinDays: 14,
		MaxResults:     50,
	})
	if err != nil {
		t.Fatalf("SearchJobs failed: %v", err)
	}
	if capturedPath != "/api/v1/agent/jobs/search" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].PublicSlug != "go-engineer-acme-123" {
		t.Errorf("unexpected slug: %s", jobs[0].PublicSlug)
	}
	if jobs[0].Description != "## Role\nBuild Go services." {
		t.Errorf("unexpected description: %s", jobs[0].Description)
	}

	expectedParams := []string{
		"cities=Prague",
		"category=backend%2Csoftware_engineering",
		"open_within_days=14",
		"description_format=markdown",
	}
	for _, exp := range expectedParams {
		if !strings.Contains(capturedQuery, exp) {
			t.Errorf("query %q missing expected %q", capturedQuery, exp)
		}
	}
}
