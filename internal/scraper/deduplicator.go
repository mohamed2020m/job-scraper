package scraper

import (
	"crypto/md5"
	"fmt"
	"job-scraper-go/internal/models"
	"strings"
	"sync"
)

// Deduplicator removes duplicate jobs based on various criteria
type Deduplicator struct {
	seenJobs map[string]bool
	mu       sync.RWMutex
}

// NewDeduplicator creates a new deduplicator
func NewDeduplicator() *Deduplicator {
	return &Deduplicator{
		seenJobs: make(map[string]bool),
	}
}

// RemoveDuplicates removes duplicate jobs from a slice
func (d *Deduplicator) RemoveDuplicates(jobs []models.Job) []models.Job {
	d.mu.Lock()
	defer d.mu.Unlock()

	var uniqueJobs []models.Job

	for _, job := range jobs {
		hash := d.generateJobHash(job)

		if !d.seenJobs[hash] {
			d.seenJobs[hash] = true
			uniqueJobs = append(uniqueJobs, job)
		}
	}

	return uniqueJobs
}

// generateJobHash creates a hash for a job based on title, company, and location
func (d *Deduplicator) generateJobHash(job models.Job) string {
	// Normalize strings for better matching
	title := strings.ToLower(strings.TrimSpace(job.Title))
	company := strings.ToLower(strings.TrimSpace(job.Company))
	location := strings.ToLower(strings.TrimSpace(job.Location))

	// Create composite key
	key := fmt.Sprintf("%s|%s|%s", title, company, location)

	// Generate MD5 hash
	hash := md5.Sum([]byte(key))
	return fmt.Sprintf("%x", hash)
}

// IsDuplicate checks if a job is a duplicate without adding it to the seen jobs
func (d *Deduplicator) IsDuplicate(job models.Job) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	hash := d.generateJobHash(job)
	return d.seenJobs[hash]
}

// Reset clears all seen jobs
func (d *Deduplicator) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.seenJobs = make(map[string]bool)
}

// GetSeenCount returns the number of unique jobs seen
func (d *Deduplicator) GetSeenCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return len(d.seenJobs)
}

// JobSimilarity represents similarity between two jobs
type JobSimilarity struct {
	Job1       models.Job
	Job2       models.Job
	Similarity float64
}

// FindSimilarJobs finds jobs that are similar but not exact duplicates
func (d *Deduplicator) FindSimilarJobs(jobs []models.Job, threshold float64) []JobSimilarity {
	var similarities []JobSimilarity

	for i := 0; i < len(jobs); i++ {
		for j := i + 1; j < len(jobs); j++ {
			similarity := d.calculateSimilarity(jobs[i], jobs[j])

			if similarity >= threshold && similarity < 1.0 {
				similarities = append(similarities, JobSimilarity{
					Job1:       jobs[i],
					Job2:       jobs[j],
					Similarity: similarity,
				})
			}
		}
	}

	return similarities
}

// calculateSimilarity calculates similarity between two jobs (0.0 to 1.0)
func (d *Deduplicator) calculateSimilarity(job1, job2 models.Job) float64 {
	// Simple similarity based on string matching
	titleSim := d.stringSimilarity(job1.Title, job2.Title)
	companySim := d.stringSimilarity(job1.Company, job2.Company)
	locationSim := d.stringSimilarity(job1.Location, job2.Location)

	// Weighted average
	return (titleSim*0.5 + companySim*0.3 + locationSim*0.2)
}

// stringSimilarity calculates similarity between two strings using Jaccard similarity
func (d *Deduplicator) stringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	if s1 == "" || s2 == "" {
		return 0.0
	}

	// Convert to lowercase and split into words
	words1 := strings.Fields(strings.ToLower(s1))
	words2 := strings.Fields(strings.ToLower(s2))

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Create sets
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, word := range words1 {
		set1[word] = true
	}

	for _, word := range words2 {
		set2[word] = true
	}

	// Calculate Jaccard similarity
	intersection := 0
	union := len(set1)

	for word := range set2 {
		if set1[word] {
			intersection++
		} else {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}
