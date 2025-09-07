package models

import "time"

type Job struct {
	ID          int        `json:"id,omitempty"`
	Title       string     `json:"title"`
	Company     string     `json:"company"`
	Location    string     `json:"location"`
	URL         string     `json:"url"`
	Description string     `json:"description,omitempty"`
	Salary      string     `json:"salary,omitempty"`
	PostedDate  *time.Time `json:"posted_date,omitempty"`
	Source      string     `json:"source"`
	JobCategory string     `json:"job_category,omitempty"`
	JobType     string     `json:"job_type,omitempty"` // full-time, part-time, contract, freelance
	ScrapedAt   time.Time  `json:"scraped_at"`
}

// JobType constants (renamed from ContractType)
const (
	JobTypeFullTime  = "full-time"
	JobTypePartTime  = "part-time"
	JobTypeContract  = "contract"
	JobTypeFreelance = "freelance"
)
