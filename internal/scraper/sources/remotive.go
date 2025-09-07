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

// RemotiveSource implements JobSource for Remotive API
type RemotiveSource struct {
	client  *httpclient.HttpClient
	baseURL string
}

// NewRemotiveSource creates a new Remotive source
func NewRemotiveSource(client *httpclient.HttpClient) *RemotiveSource {
	return &RemotiveSource{
		client:  client,
		baseURL: "https://remotive.com/api/remote-jobs",
	}
}

func (r *RemotiveSource) GetName() string {
	return "Remotive"
}

func (r *RemotiveSource) GetRateLimit() int {
	return 100 // 100 requests per minute
}

func (r *RemotiveSource) SupportsSearch() bool {
	return true
}

func (r *RemotiveSource) GetBaseURL() string {
	return r.baseURL
}

// RemotiveResponse represents the API response from Remotive
type RemotiveResponse struct {
	Jobs []RemotiveJob `json:"jobs"`
}

// RemotiveJob represents a job from Remotive API
type RemotiveJob struct {
	ID                        int    `json:"id"`
	URL                       string `json:"url"`
	Title                     string `json:"title"`
	CompanyName               string `json:"company_name"`
	CompanyLogo               string `json:"company_logo"`
	Category                  string `json:"category"`
	JobType                   string `json:"job_type"`
	PublicationDate           string `json:"publication_date"`
	CandidateRequiredLocation string `json:"candidate_required_location"`
	Salary                    string `json:"salary"`
	Description               string `json:"description"`
}

func (r *RemotiveSource) FetchJobs() ([]models.Job, error) {
	resp, err := r.client.Get(r.baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Remotive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Remotive API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response RemotiveResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Remotive response: %w", err)
	}

	var jobs []models.Job
	for _, remotiveJob := range response.Jobs {
		location := remotiveJob.CandidateRequiredLocation
		if location == "" {
			location = "Remote"
		}

		// Parse posted date with better error handling
		var postedDate *time.Time
		if remotiveJob.PublicationDate != "" {
			// Try multiple date formats that Remotive might use
			formats := []string{
				"2006-01-02",
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05-07:00",
				"2006-01-02 15:04:05",
			}

			for _, format := range formats {
				if parsed, err := time.Parse(format, remotiveJob.PublicationDate); err == nil {
					postedDate = &parsed
					break
				}
			}

			// If all parsing attempts fail, set to current time as fallback
			if postedDate == nil {
				now := time.Now()
				postedDate = &now
			}
		}

		// Use job_type directly from Remotive API
		jobType := r.getJobType(remotiveJob.JobType)

		// Ensure all fields have values, even if empty, to maintain consistent JSON structure
		description := remotiveJob.Description
		if description == "" {
			description = " " // Single space instead of empty to avoid omitempty
		}
		salary := remotiveJob.Salary
		if salary == "" {
			salary = " " // Single space instead of empty to avoid omitempty
		}
		jobCategory := r.getJobCategory(remotiveJob.Category, remotiveJob.Title)
		if jobCategory == "" {
			jobCategory = " " // Single space instead of empty
		}

		job := models.Job{
			Title:       remotiveJob.Title,
			Company:     remotiveJob.CompanyName,
			Location:    location,
			URL:         remotiveJob.URL,
			Description: description,
			Salary:      salary,
			PostedDate:  postedDate,
			Source:      r.GetName(),
			JobCategory: jobCategory,
			JobType:     jobType,
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// FetchJobsByCategory fetches jobs from specific category
func (r *RemotiveSource) FetchJobsByCategory(category string) ([]models.Job, error) {
	url := fmt.Sprintf("%s?category=%s", r.baseURL, strings.ToLower(category))

	resp, err := r.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Remotive with category %s: %w", category, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Remotive API returned status %d for category %s", resp.StatusCode, category)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response RemotiveResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Remotive response: %w", err)
	}

	var jobs []models.Job
	for _, remotiveJob := range response.Jobs {
		location := remotiveJob.CandidateRequiredLocation
		if location == "" {
			location = "Remote"
		}

		// Parse posted date with better error handling
		var postedDate *time.Time
		if remotiveJob.PublicationDate != "" {
			// Try multiple date formats that Remotive might use
			formats := []string{
				"2006-01-02",
				"2006-01-02T15:04:05Z",
				"2006-01-02T15:04:05-07:00",
				"2006-01-02 15:04:05",
			}

			for _, format := range formats {
				if parsed, err := time.Parse(format, remotiveJob.PublicationDate); err == nil {
					postedDate = &parsed
					break
				}
			}

			// If all parsing attempts fail, set to current time as fallback
			if postedDate == nil {
				now := time.Now()
				postedDate = &now
			}
		}

		// Ensure all fields have values for consistent JSON structure
		description := remotiveJob.Description
		if description == "" {
			description = " "
		}
		salary := remotiveJob.Salary
		if salary == "" {
			salary = " "
		}
		jobCategory := r.getJobCategory(remotiveJob.Category, remotiveJob.Title)
		if jobCategory == "" {
			jobCategory = " "
		}

		job := models.Job{
			Title:       remotiveJob.Title,
			Company:     remotiveJob.CompanyName,
			Location:    location,
			URL:         remotiveJob.URL,
			Description: description,
			Salary:      salary,
			PostedDate:  postedDate,
			Source:      r.GetName(),
			JobCategory: jobCategory,
			JobType:     r.getJobType(remotiveJob.JobType),
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// getJobCategory determines job category from Remotive category and title
func (r *RemotiveSource) getJobCategory(category, title string) string {
	// Use category if available
	if category != "" {
		// Replace hyphens with spaces and capitalize first letter of each word
		formatted := strings.ReplaceAll(category, "-", " ")
		words := strings.Fields(formatted)
		for i, word := range words {
			if len(word) > 0 {
				words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
			}
		}
		return strings.Join(words, " ")
	}

	// Otherwise infer from title
	title = strings.ToLower(title)
	if strings.Contains(title, "frontend") || strings.Contains(title, "react") || strings.Contains(title, "vue") || strings.Contains(title, "angular") {
		return "Frontend Development"
	}
	if strings.Contains(title, "backend") || strings.Contains(title, "golang") || strings.Contains(title, "go") || strings.Contains(title, "python") {
		return "Backend Development"
	}
	if strings.Contains(title, "fullstack") || strings.Contains(title, "full-stack") || strings.Contains(title, "full stack") {
		return "Full Stack Development"
	}
	if strings.Contains(title, "devops") || strings.Contains(title, "sre") {
		return "DevOps"
	}
	if strings.Contains(title, "data") || strings.Contains(title, "analyst") {
		return "Data Science"
	}
	if strings.Contains(title, "mobile") || strings.Contains(title, "ios") || strings.Contains(title, "android") {
		return "Mobile Development"
	}
	if strings.Contains(title, "design") || strings.Contains(title, "ux") || strings.Contains(title, "ui") {
		return "Design"
	}
	return "Technology"
}

// getJobType maps Remotive job types to our standardized job types
func (r *RemotiveSource) getJobType(jobType string) string {
	jobTypeLower := strings.ToLower(jobType)

	if strings.Contains(jobTypeLower, "full_time") || strings.Contains(jobTypeLower, "full-time") {
		return models.JobTypeFullTime
	}
	if strings.Contains(jobTypeLower, "part_time") || strings.Contains(jobTypeLower, "part-time") {
		return models.JobTypePartTime
	}
	if strings.Contains(jobTypeLower, "contract") {
		return models.JobTypeContract
	}
	if strings.Contains(jobTypeLower, "freelance") {
		return models.JobTypeFreelance
	}

	return models.JobTypeFullTime // Default
}
