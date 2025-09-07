package storage

import (
	"fmt"
	"os"
	"time"

	supabase "github.com/nedpals/supabase-go"

	"job-scraper-go/internal/models"
)

// SupabaseStore uses the nedpals/supabase-go SDK to persist jobs.
type SupabaseStore struct {
	client *supabase.Client
}

// NewSupabaseStore creates a SupabaseStore. It reads SUPABASE_URL and SUPABASE_KEY
// from environment variables if empty values are provided.
func NewSupabaseStore(supabaseURL, supabaseKey string) (*SupabaseStore, error) {
	if supabaseURL == "" {
		supabaseURL = os.Getenv("SUPABASE_URL")
	}
	if supabaseKey == "" {
		supabaseKey = os.Getenv("SUPABASE_KEY")
	}
	if supabaseURL == "" || supabaseKey == "" {
		return nil, fmt.Errorf("supabase URL and key must be provided via args or SUPABASE_URL / SUPABASE_KEY env vars")
	}

	// CreateClient returns *supabase.Client (no error)
	client := supabase.CreateClient(supabaseURL, supabaseKey)
	return &SupabaseStore{client: client}, nil
}

func (s *SupabaseStore) SaveJob(job *models.Job) error {
	// Set scraped_at timestamp if not already set
	if job.ScrapedAt.IsZero() {
		job.ScrapedAt = time.Now()
	}

	// Use the SDK's DB wrapper: client.DB.From(...).Insert(...).Execute(&results)
	var results []models.Job
	// Insert expects a value (not pointer) in examples
	err := s.client.DB.From("jobs").Insert(*job).Execute(&results)
	return err
}

func (s *SupabaseStore) GetJobs() ([]models.Job, error) {
	var res []models.Job
	err := s.client.DB.From("jobs").Select("*").Execute(&res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// SaveJobs saves multiple jobs in a single batch operation for better performance
func (s *SupabaseStore) SaveJobs(jobs []models.Job) error {
	if len(jobs) == 0 {
		return nil
	}

	// Set scraped_at timestamp for all jobs
	now := time.Now()
	for i := range jobs {
		if jobs[i].ScrapedAt.IsZero() {
			jobs[i].ScrapedAt = now
		}
	}

	// Use batch insert
	var results []models.Job
	err := s.client.DB.From("jobs").Insert(jobs).Execute(&results)
	return err
}
