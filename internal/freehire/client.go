package freehire

import (
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

// ValidCategories contains all valid Freehire `category` facet values from the OpenAPI spec.
var ValidCategories = map[string]bool{
	"software_engineering":   true,
	"backend":                true,
	"frontend":               true,
	"fullstack":              true,
	"mobile":                 true,
	"devops":                 true,
	"sre":                    true,
	"network_engineering":    true,
	"it_support":             true,
	"data_engineering":       true,
	"data_science":           true,
	"data_analytics":         true,
	"ml_ai":                  true,
	"ai_engineering":         true,
	"qa":                     true,
	"security":               true,
	"hardware":               true,
	"embedded":               true,
	"blockchain":             true,
	"architecture":           true,
	"design":                 true,
	"creative":               true,
	"engineering_design":     true,
	"industrial_engineering": true,
	"occupational_safety":    true,
	"product":                true,
	"project_management":     true,
	"healthcare":             true,
	"skilled_trades":         true,
	"retail":                 true,
	"hospitality":            true,
	"logistics":              true,
	"education":              true,
	"personal_services":      true,
	"administration":         true,
	"social_services":        true,
	"science":                true,
	"manufacturing":          true,
	"public_safety":          true,
	"marketing":              true,
	"sales":                  true,
	"support":                true,
	"business_analysis":      true,
	"solutions_engineering":  true,
	"developer_relations":    true,
	"technical_writing":      true,
	"recruiting":             true,
	"hr":                     true,
	"finance":                true,
	"legal":                  true,
	"operations":             true,
	"customer_success":       true,
	"other":                  true,
}

type Enrichment struct {
	Summary            string   `json:"summary"`
	Seniority          string   `json:"seniority"`
	Category           string   `json:"category"`
	EmploymentType     string   `json:"employment_type"`
	Relocation         string   `json:"relocation"`
	VisaSponsorship    bool     `json:"visa_sponsorship"`
	ExperienceYearsMin *int     `json:"experience_years_min"`
	EnglishLevel       string   `json:"english_level"`
	Domains            []string `json:"domains"`
	CompanyType        string   `json:"company_type"`
	CompanySize        string   `json:"company_size"`
	SalaryMin          *int     `json:"salary_min"`
	SalaryMax          *int     `json:"salary_max"`
	SalaryCurrency     string   `json:"salary_currency"`
	SalaryPeriod       string   `json:"salary_period"`
}

type Job struct {
	PublicSlug  string     `json:"public_slug"`
	Source      string     `json:"source"`
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Company     string     `json:"company"`
	CompanySlug string     `json:"company_slug"`
	Location    string     `json:"location"`
	Description string     `json:"description"`
	WorkMode    string     `json:"work_mode"`
	Skills      []string   `json:"skills"`
	Cities      []string   `json:"cities"`
	PostedAt    *time.Time `json:"posted_at"`
	CreatedAt   *time.Time `json:"created_at"`
	Enrichment  Enrichment `json:"enrichment"`
}

type Meta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type searchResponse struct {
	Data  []Job  `json:"data"`
	Meta  Meta   `json:"meta"`
	Error string `json:"error"`
}

type detailResponse struct {
	Data  Job    `json:"data"`
	Error string `json:"error"`
}

type SearchParams struct {
	City           string
	Categories     []string
	OpenWithinDays int
	MaxResults     int
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  strings.TrimSpace(apiKey),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchJobs queries GET /agent/jobs/search to fetch jobs with untruncated markdown descriptions.
func (c *Client) SearchJobs(ctx context.Context, params SearchParams) ([]Job, error) {
	maxResults := params.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	var allJobs []Job
	offset := 0
	pageSize := 100
	if maxResults < pageSize {
		pageSize = maxResults
	}

	for len(allJobs) < maxResults {
		q := url.Values{}
		if params.City != "" {
			q.Set("cities", params.City)
		}
		if len(params.Categories) > 0 {
			q.Set("category", strings.Join(params.Categories, ","))
		}
		if params.OpenWithinDays > 0 {
			q.Set("open_within_days", strconv.Itoa(params.OpenWithinDays))
		}
		q.Set("description_format", "markdown")
		q.Set("limit", strconv.Itoa(pageSize))
		q.Set("offset", strconv.Itoa(offset))

		endpoint := fmt.Sprintf("%s/agent/jobs/search?%s", c.baseURL, q.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("execute search request: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			var errResp searchResponse
			_ = json.Unmarshal(body, &errResp)
			if errResp.Error != "" {
				return nil, fmt.Errorf("freehire API error (%d): %s", resp.StatusCode, errResp.Error)
			}
			return nil, fmt.Errorf("freehire API status %d: %s", resp.StatusCode, string(body))
		}

		var parsed searchResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("decode search response: %w", err)
		}

		allJobs = append(allJobs, parsed.Data...)
		if len(parsed.Data) < pageSize || len(allJobs) >= parsed.Meta.Total {
			break
		}
		offset += len(parsed.Data)
		if offset+pageSize > 10000 {
			break
		}
	}

	if len(allJobs) > maxResults {
		allJobs = allJobs[:maxResults]
	}
	return allJobs, nil
}

// GetJob fetches a single job by slug via /jobs/{slug}.
func (c *Client) GetJob(ctx context.Context, slug string) (*Job, error) {
	endpoint := fmt.Sprintf("%s/jobs/%s", c.baseURL, url.PathEscape(slug))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build get job request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute get job request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read job detail response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp detailResponse
		_ = json.Unmarshal(body, &errResp)
		if errResp.Error != "" {
			return nil, fmt.Errorf("freehire API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("freehire API status %d: %s", resp.StatusCode, string(body))
	}

	var parsed detailResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode job detail response: %w", err)
	}
	return &parsed.Data, nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "autojob-bot/1.0")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}
