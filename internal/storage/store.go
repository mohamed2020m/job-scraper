package storage

import "job-scraper-go/internal/models"

type Store interface {
	SaveJob(job *models.Job) error
	SaveJobs(jobs []models.Job) error // Batch save for better performance
	GetJobs() ([]models.Job, error)
}
