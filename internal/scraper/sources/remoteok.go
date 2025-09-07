package sources

import (
	"encoding/json"
	"fmt"
	"io"
	"job-scraper-go/internal/models"
	"job-scraper-go/pkg/httpclient"
	"net/http"
	"strings"
	"time"
)

// RemoteOKSource implements JobSource for RemoteOK API
type RemoteOKSource struct {
	client  *httpclient.HttpClient
	baseURL string
}

// NewRemoteOKSource creates a new RemoteOK source
func NewRemoteOKSource(client *httpclient.HttpClient) *RemoteOKSource {
	return &RemoteOKSource{
		client:  client,
		baseURL: "https://remoteok.com/api",
	}
}

func (r *RemoteOKSource) GetName() string {
	return "RemoteOK"
}

func (r *RemoteOKSource) GetRateLimit() int {
	return 60 // 60 requests per minute
}

func (r *RemoteOKSource) SupportsSearch() bool {
	return true
}

func (r *RemoteOKSource) GetBaseURL() string {
	return r.baseURL
}

// RemoteOKJob represents a job from RemoteOK API
type RemoteOKJob struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Company     string    `json:"company"`
	CompanyLogo string    `json:"company_logo"`
	Position    string    `json:"position"`
	Tags        []string  `json:"tags"`
	Description string    `json:"description"`
	Location    string    `json:"location"`
	Original    bool      `json:"original"`
	URL         string    `json:"url"`
	ApplyURL    string    `json:"apply_url"`
	Date        time.Time `json:"date"`
}

func (r *RemoteOKSource) FetchJobs() ([]models.Job, error) {
	resp, err := r.client.Get(r.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from RemoteOK: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RemoteOK API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var remoteOKJobs []RemoteOKJob
	if err := json.Unmarshal(body, &remoteOKJobs); err != nil {
		return nil, fmt.Errorf("failed to parse RemoteOK response: %w", err)
	}

	var jobs []models.Job
	for _, remoteJob := range remoteOKJobs {
		// Skip the first element which is metadata
		if remoteJob.ID == "" {
			continue
		}

		// Extract job type from tags
		jobType := r.getJobType(remoteJob.Tags)

		job := models.Job{
			Title:       remoteJob.Position,
			Company:     remoteJob.Company,
			Location:    remoteJob.Location,
			URL:         remoteJob.URL,
			Description: remoteJob.Description,
			Salary:      "", // RemoteOK doesn't provide salary information
			PostedDate:  &remoteJob.Date,
			Source:      r.GetName(),
			JobCategory: r.getJobCategory(remoteJob.Tags),
			JobType:     jobType,
		}

		if job.URL == "" {
			job.URL = fmt.Sprintf("https://remoteok.com/remote-jobs/%s", remoteJob.Slug)
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// getJobCategory extracts job category from tags - using a conservative approach since tags are mixed
func (r *RemoteOKSource) getJobCategory(tags []string) string {
	// Priority mapping - more specific tags first
	categoryMap := map[string]string{
		"backend":    "Backend Development",
		"frontend":   "Frontend Development",
		"fullstack":  "Full Stack Development",
		"full-stack": "Full Stack Development",
		"devops":     "DevOps",
		"data":       "Data Science",
		"ml":         "Machine Learning",
		"ai":         "Artificial Intelligence",
		"mobile":     "Mobile Development",
		"ios":        "Mobile Development",
		"android":    "Mobile Development",
		"design":     "Design",
		"marketing":  "Marketing",
		"sales":      "Sales",
	}

	// First pass - look for explicit category tags
	for _, tag := range tags {
		if category, exists := categoryMap[strings.ToLower(tag)]; exists {
			return category
		}
	}

	// Second pass - infer from technology tags
	techMap := map[string]string{
		"golang":     "Backend Development",
		"go":         "Backend Development",
		"python":     "Backend Development",
		"java":       "Backend Development",
		"javascript": "Frontend Development",
		"react":      "Frontend Development",
		"vue":        "Frontend Development",
		"angular":    "Frontend Development",
	}

	for _, tag := range tags {
		if category, exists := techMap[strings.ToLower(tag)]; exists {
			return category
		}
	}

	return "Technology" // Safe default
}

// getJobType extracts job type from tags
func (r *RemoteOKSource) getJobType(tags []string) string {
	for _, tag := range tags {
		switch strings.ToLower(tag) {
		case "full-time", "fulltime", "permanent":
			return models.JobTypeFullTime
		case "part-time", "parttime":
			return models.JobTypePartTime
		case "contract", "contractor", "freelance":
			return models.JobTypeContract
		case "internship", "intern":
			return "internship"
		}
	}
	return models.JobTypeFullTime // Default assumption
}
