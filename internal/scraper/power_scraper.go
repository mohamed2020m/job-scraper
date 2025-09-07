package scraper

import (
	"context"
	"fmt"
	"job-scraper-go/internal/models"
	"job-scraper-go/internal/scraper/sources"
	"job-scraper-go/internal/storage"
	"job-scraper-go/pkg/httpclient"
	"log"
	"sync"
	"time"
)

// PowerScraper is an enhanced scraper with concurrent processing and rate limiting
type PowerScraper struct {
	sourceManager *sources.SourceManager
	storage       storage.Store
	client        *httpclient.HttpClient
	rateLimiter   *RateLimiter
	deduplicator  *Deduplicator
	retryConfig   RetryConfig
	metrics       *ScraperMetrics
	logger        *log.Logger
}

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// ScraperMetrics tracks scraper performance
type ScraperMetrics struct {
	TotalJobsScraped  int64
	TotalJobsSaved    int64
	TotalDuplicates   int64
	TotalErrors       int64
	ScrapingDuration  time.Duration
	SourcePerformance map[string]SourceMetrics
	mu                sync.RWMutex
}

// SourceMetrics tracks performance per source
type SourceMetrics struct {
	JobsScraped  int64
	JobsSaved    int64
	Duplicates   int64
	Errors       int64
	ResponseTime time.Duration
	LastScraped  time.Time
}

// NewPowerScraper creates a new enhanced scraper
func NewPowerScraper(storage storage.Store, client *httpclient.HttpClient, logger *log.Logger) *PowerScraper {
	return &PowerScraper{
		sourceManager: sources.NewSourceManager(),
		storage:       storage,
		client:        client,
		rateLimiter:   NewRateLimiter(),
		deduplicator:  NewDeduplicator(),
		retryConfig: RetryConfig{
			MaxRetries:    3,
			InitialDelay:  1 * time.Second,
			MaxDelay:      30 * time.Second,
			BackoffFactor: 2.0,
		},
		metrics: &ScraperMetrics{
			SourcePerformance: make(map[string]SourceMetrics),
		},
		logger: logger,
	}
}

// InitializeSources sets up all available job sources
func (ps *PowerScraper) InitializeSources() {
	// Register RemoteOK
	remoteOK := sources.NewRemoteOKSource(ps.client)
	ps.sourceManager.RegisterSource(remoteOK, sources.JobSourceConfig{
		Enabled:   true,
		RateLimit: remoteOK.GetRateLimit(),
	})

	// Register Remotive
	remotive := sources.NewRemotiveSource(ps.client)
	ps.sourceManager.RegisterSource(remotive, sources.JobSourceConfig{
		Enabled:   true,
		RateLimit: remotive.GetRateLimit(),
	})

	ps.logger.Printf("Initialized %d job sources", len(ps.sourceManager.GetEnabledSources()))
}

// ScrapeAllSources scrapes jobs from all enabled sources concurrently
func (ps *PowerScraper) ScrapeAllSources(ctx context.Context) error {
	startTime := time.Now()
	defer func() {
		ps.metrics.mu.Lock()
		ps.metrics.ScrapingDuration = time.Since(startTime)
		ps.metrics.mu.Unlock()
	}()

	enabledSources := ps.sourceManager.GetEnabledSources()
	if len(enabledSources) == 0 {
		return fmt.Errorf("no enabled sources found")
	}

	// Channel to collect results from all sources
	resultsChan := make(chan ScraperResult, len(enabledSources))

	// Worker pool for concurrent scraping
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent sources

	for name, source := range enabledSources {
		wg.Add(1)
		go func(sourceName string, jobSource sources.JobSource) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := ps.scrapeSource(ctx, sourceName, jobSource)
			resultsChan <- result
		}(name, source)
	}

	// Close results channel when all workers finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect and process results
	var allJobs []models.Job
	for result := range resultsChan {
		if result.Error != nil {
			ps.metrics.mu.Lock()
			ps.metrics.TotalErrors++
			ps.metrics.mu.Unlock()
			ps.logger.Printf("Error scraping %s: %v", result.Source, result.Error)
			continue
		}

		// Deduplicate jobs
		uniqueJobs := ps.deduplicator.RemoveDuplicates(result.Jobs)
		duplicates := len(result.Jobs) - len(uniqueJobs)

		allJobs = append(allJobs, uniqueJobs...)

		// Update metrics
		ps.metrics.mu.Lock()
		ps.metrics.TotalJobsScraped += int64(len(result.Jobs))
		ps.metrics.TotalDuplicates += int64(duplicates)

		sourceMetric := ps.metrics.SourcePerformance[result.Source]
		sourceMetric.JobsScraped = int64(len(result.Jobs))
		sourceMetric.Duplicates = int64(duplicates)
		sourceMetric.ResponseTime = result.Duration
		sourceMetric.LastScraped = time.Now()
		ps.metrics.SourcePerformance[result.Source] = sourceMetric
		ps.metrics.mu.Unlock()

		ps.logger.Printf("Scraped %d jobs from %s (%d unique, %d duplicates) in %v",
			len(result.Jobs), result.Source, len(uniqueJobs), duplicates, result.Duration)
	}

	// Save all unique jobs to storage
	if len(allJobs) > 0 {
		if err := ps.saveJobs(ctx, allJobs); err != nil {
			return fmt.Errorf("failed to save jobs: %w", err)
		}

		ps.metrics.mu.Lock()
		ps.metrics.TotalJobsSaved = int64(len(allJobs))
		ps.metrics.mu.Unlock()
	}

	ps.logger.Printf("Scraping completed: %d total jobs, %d saved, %d duplicates in %v",
		ps.metrics.TotalJobsScraped, ps.metrics.TotalJobsSaved,
		ps.metrics.TotalDuplicates, ps.metrics.ScrapingDuration)

	return nil
}

// ScraperResult holds the result from scraping a single source
type ScraperResult struct {
	Source   string
	Jobs     []models.Job
	Error    error
	Duration time.Duration
}

// scrapeSource scrapes jobs from a single source with rate limiting and retries
func (ps *PowerScraper) scrapeSource(ctx context.Context, sourceName string, source sources.JobSource) ScraperResult {
	startTime := time.Now()

	// Apply rate limiting
	config, _ := ps.sourceManager.GetSourceConfig(sourceName)
	if err := ps.rateLimiter.Wait(ctx, sourceName, config.RateLimit); err != nil {
		return ScraperResult{
			Source:   sourceName,
			Error:    fmt.Errorf("rate limit error: %w", err),
			Duration: time.Since(startTime),
		}
	}

	// Attempt scraping with retries
	var jobs []models.Job
	var lastError error

	for attempt := 0; attempt <= ps.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := ps.calculateBackoffDelay(attempt)
			ps.logger.Printf("Retrying %s (attempt %d/%d) after %v",
				sourceName, attempt+1, ps.retryConfig.MaxRetries+1, delay)

			select {
			case <-ctx.Done():
				return ScraperResult{
					Source:   sourceName,
					Error:    ctx.Err(),
					Duration: time.Since(startTime),
				}
			case <-time.After(delay):
			}
		}

		jobs, lastError = source.FetchJobs()
		if lastError == nil {
			break
		}

		ps.logger.Printf("Attempt %d failed for %s: %v", attempt+1, sourceName, lastError)
	}

	if lastError != nil {
		ps.metrics.mu.Lock()
		sourceMetric := ps.metrics.SourcePerformance[sourceName]
		sourceMetric.Errors++
		ps.metrics.SourcePerformance[sourceName] = sourceMetric
		ps.metrics.mu.Unlock()
	}

	return ScraperResult{
		Source:   sourceName,
		Jobs:     jobs,
		Error:    lastError,
		Duration: time.Since(startTime),
	}
}

// calculateBackoffDelay calculates exponential backoff delay
func (ps *PowerScraper) calculateBackoffDelay(attempt int) time.Duration {
	delay := time.Duration(float64(ps.retryConfig.InitialDelay) *
		float64(attempt) * ps.retryConfig.BackoffFactor)

	if delay > ps.retryConfig.MaxDelay {
		delay = ps.retryConfig.MaxDelay
	}

	return delay
}

// saveJobs saves jobs to storage with batch processing
func (ps *PowerScraper) saveJobs(ctx context.Context, jobs []models.Job) error {
	const batchSize = 50

	for i := 0; i < len(jobs); i += batchSize {
		end := i + batchSize
		if end > len(jobs) {
			end = len(jobs)
		}

		batch := jobs[i:end]

		// Try batch save first for better performance
		if err := ps.storage.SaveJobs(batch); err != nil {
			ps.logger.Printf("Batch save failed, falling back to individual saves: %v", err)
			// Fall back to individual saves if batch fails
			for _, job := range batch {
				if err := ps.storage.SaveJob(&job); err != nil {
					ps.logger.Printf("Failed to save job %s at %s: %v", job.Title, job.Company, err)
					// Continue with other jobs instead of failing completely
					continue
				}
			}
		}

		// Check if context was cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// GetMetrics returns current scraper metrics
func (ps *PowerScraper) GetMetrics() ScraperMetrics {
	ps.metrics.mu.RLock()
	defer ps.metrics.mu.RUnlock()

	// Create a copy to avoid race conditions - without copying the mutex
	sourcePerformance := make(map[string]SourceMetrics)
	for k, v := range ps.metrics.SourcePerformance {
		sourcePerformance[k] = v
	}

	return ScraperMetrics{
		TotalJobsScraped:  ps.metrics.TotalJobsScraped,
		TotalJobsSaved:    ps.metrics.TotalJobsSaved,
		TotalDuplicates:   ps.metrics.TotalDuplicates,
		TotalErrors:       ps.metrics.TotalErrors,
		ScrapingDuration:  ps.metrics.ScrapingDuration,
		SourcePerformance: sourcePerformance,
	}
}
