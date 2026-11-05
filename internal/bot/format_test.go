package bot

import (
	"strings"
	"testing"

	"autojob/internal/freehire"
)

func TestFormatJobPageAndPagination(t *testing.T) {
	store := NewSessionStore()

	jobs := []freehire.Job{
		{PublicSlug: "job-1", Title: "Go Backend Dev", Company: "Alpha", Location: "Prague", WorkMode: "hybrid", URL: "https://example.com/1"},
		{PublicSlug: "job-2", Title: "Platform Engineer", Company: "Beta", Location: "Prague", WorkMode: "onsite", URL: "https://example.com/2"},
		{PublicSlug: "job-3", Title: "SRE", Company: "Gamma", Location: "Prague", URL: "https://example.com/3"},
		{PublicSlug: "job-4", Title: "Staff Engineer", Company: "Delta", Location: "Prague", URL: "https://example.com/4"},
		{PublicSlug: "job-5", Title: "Software Engineer", Company: "Epsilon", Location: "Prague", URL: "https://example.com/5"},
	}

	seen := map[string]bool{"job-2": true}
	sess := store.Create(12345, "all", "Prague", []string{"backend"}, 7, jobs, seen)

	if sess.TotalPages() != 2 {
		t.Fatalf("expected 2 total pages, got %d", sess.TotalPages())
	}

	page0 := FormatJobPage(sess, 0)
	if !strings.Contains(page0, "Page <b>1/2</b>") {
		t.Errorf("expected Page 1/2 in output, got:\n%s", page0)
	}
	if !strings.Contains(page0, "<b>2. Platform Engineer</b> — <b>Beta</b> 👀") {
		t.Errorf("expected seen badge 👀 on job 2, got:\n%s", page0)
	}
	if strings.Contains(page0, "Epsilon") {
		t.Errorf("did not expect 5th job on page 0")
	}

	page1 := FormatJobPage(sess, 1)
	if !strings.Contains(page1, "Epsilon") {
		t.Errorf("expected 5th job on page 1, got:\n%s", page1)
	}
}
