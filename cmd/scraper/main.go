package main

import (
	"context"
	"fmt"
	"job-scraper-go/internal/config"
	"job-scraper-go/internal/scraper"
	"job-scraper-go/internal/storage"
	"job-scraper-go/pkg/httpclient"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Setup logging
	logger, logFile, err := setupLogging(cfg.Monitoring.LogFile, cfg.Monitoring.LogLevel)
	if err != nil {
		log.Fatalf("Failed to setup logging: %v", err)
	}
	if logFile != nil {
		defer logFile.Close()
	}

	logger.Printf("Starting Job Scraper with %d concurrent sources", cfg.Scraper.ConcurrentSources)

	// Initialize HTTP client
	httpClient := httpclient.NewHttpClient(cfg.Scraper.RequestTimeout)

	// Initialize storage
	store, err := storage.NewSupabaseStore(cfg.Database.SupabaseURL, cfg.Database.SupabaseKey)
	if err != nil {
		logger.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize power scraper
	powerScraper := scraper.NewPowerScraper(store, httpClient, logger)
	powerScraper.InitializeSources()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start background scraping if interval is configured
	var scraperDone chan struct{}
	if cfg.Scraper.ScrapingInterval > 0 {
		scraperDone = make(chan struct{})
		go runPeriodicScraping(ctx, powerScraper, cfg.Scraper.ScrapingInterval, logger, scraperDone)
	}

	// Start metrics reporting if monitoring is enabled
	var metricsDone chan struct{}
	if cfg.Monitoring.Enabled {
		metricsDone = make(chan struct{})
		go runMetricsReporting(ctx, powerScraper, cfg.Monitoring.MetricsInterval, logger, metricsDone)
	}

	// Run initial scraping
	logger.Println("Running initial scraping...")
	if err := powerScraper.ScrapeAllSources(ctx); err != nil {
		logger.Printf("Initial scraping failed: %v", err)
	}

	// Print initial metrics
	printMetrics(powerScraper, logger)

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		logger.Printf("Received signal %v, shutting down gracefully...", sig)
	case <-ctx.Done():
		logger.Println("Context cancelled, shutting down...")
	}

	// Cancel context to stop all background operations
	cancel()

	// Wait for background operations to complete
	if scraperDone != nil {
		<-scraperDone
		logger.Println("Periodic scraping stopped")
	}
	if metricsDone != nil {
		<-metricsDone
		logger.Println("Metrics reporting stopped")
	}

	logger.Println("Job Scraper shutdown complete")
}

// setupLogging configures logging based on the configuration
func setupLogging(logFile, logLevel string) (*log.Logger, *os.File, error) {
	var logOutput *os.File
	var err error

	if logFile != "" {
		// Ensure log directory exists
		if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
			return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		logOutput, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open log file: %w", err)
		}
	} else {
		logOutput = os.Stdout
	}

	logger := log.New(logOutput, "[SCRAPER] ", log.LstdFlags|log.Lshortfile)
	return logger, logOutput, nil
}

// runPeriodicScraping runs the scraper at regular intervals
func runPeriodicScraping(ctx context.Context, powerScraper *scraper.PowerScraper, interval time.Duration, logger *log.Logger, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Printf("Starting periodic scraping every %v", interval)

	for {
		select {
		case <-ctx.Done():
			logger.Println("Periodic scraping cancelled")
			return
		case <-ticker.C:
			logger.Println("Starting scheduled scraping...")
			start := time.Now()

			if err := powerScraper.ScrapeAllSources(ctx); err != nil {
				logger.Printf("Scheduled scraping failed: %v", err)
			} else {
				logger.Printf("Scheduled scraping completed in %v", time.Since(start))
			}

			// Print metrics after each scraping
			printMetrics(powerScraper, logger)
		}
	}
}

// runMetricsReporting periodically reports scraper metrics
func runMetricsReporting(ctx context.Context, powerScraper *scraper.PowerScraper, interval time.Duration, logger *log.Logger, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Printf("Starting metrics reporting every %v", interval)

	for {
		select {
		case <-ctx.Done():
			logger.Println("Metrics reporting cancelled")
			return
		case <-ticker.C:
			printMetrics(powerScraper, logger)
		}
	}
}

// printMetrics prints current scraper metrics
func printMetrics(powerScraper *scraper.PowerScraper, logger *log.Logger) {
	metrics := powerScraper.GetMetrics()

	logger.Printf("=== Scraper Metrics ===")
	logger.Printf("Total Jobs Scraped: %d", metrics.TotalJobsScraped)
	logger.Printf("Total Jobs Saved: %d", metrics.TotalJobsSaved)
	logger.Printf("Total Duplicates: %d", metrics.TotalDuplicates)
	logger.Printf("Total Errors: %d", metrics.TotalErrors)
	logger.Printf("Last Scraping Duration: %v", metrics.ScrapingDuration)

	if len(metrics.SourcePerformance) > 0 {
		logger.Printf("=== Source Performance ===")
		for source, perf := range metrics.SourcePerformance {
			logger.Printf("%s: scraped=%d, saved=%d, duplicates=%d, errors=%d, response_time=%v, last_scraped=%v",
				source, perf.JobsScraped, perf.JobsSaved, perf.Duplicates, perf.Errors,
				perf.ResponseTime, perf.LastScraped.Format("2006-01-02 15:04:05"))
		}
	}

	logger.Printf("========================")
}
