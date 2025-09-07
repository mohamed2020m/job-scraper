package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"job-scraper-go/internal/config"
	"job-scraper-go/internal/models"
	"job-scraper-go/internal/scraper"
	"job-scraper-go/internal/scraper/sources"
	"job-scraper-go/internal/storage"
	"job-scraper-go/pkg/httpclient"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	var (
		configFile = flag.String("config", "config.json", "Configuration file path")
		command    = flag.String("cmd", "scrape", "Command to run: scrape, metrics, test, config, sources")
		source     = flag.String("source", "", "Specific source to scrape (remoteok, remotive)")
		category   = flag.String("category", "", "Filter by category (software-dev, devops, data, etc.)")
		output     = flag.String("output", "console", "Output format: console, json")
		verbose    = flag.Bool("verbose", false, "Verbose output")
		help       = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	// Show help if requested
	if *help {
		printUsage()
		os.Exit(0)
	}

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Execute command
	switch *command {
	case "scrape":
		runScrapeCommand(cfg, *source, *category, *output, *verbose)
	case "metrics":
		runMetricsCommand(cfg, *output)
	case "test":
		runTestCommand(cfg, *source, *verbose)
	case "config":
		runConfigCommand(cfg, *output)
	case "sources":
		runSourcesCommand(cfg, *output)
	default:
		fmt.Printf("Unknown command: %s\n", *command)
		printUsage()
		os.Exit(1)
	}
}

func runScrapeCommand(cfg *config.Config, source, category, output string, verbose bool) {
	fmt.Println("Starting job scraping...")

	// Initialize components
	httpClient := httpclient.NewHttpClient(cfg.Scraper.RequestTimeout)
	store, err := storage.NewSupabaseStore(cfg.Database.SupabaseURL, cfg.Database.SupabaseKey)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)
	if !verbose {
		logger.SetOutput(log.Writer())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var metrics *scraper.ScraperMetrics

	if source != "" {
		// Scrape specific source
		fmt.Printf("Scraping specific source: %s\n", source)
		if category != "" {
			fmt.Printf("Filtering by category: %s\n", category)
		}
		metrics = scrapeSingleSource(httpClient, store, source, category, logger, ctx)
	} else {
		// Scrape all sources
		powerScraper := scraper.NewPowerScraper(store, httpClient, logger)
		powerScraper.InitializeSources()

		if err := powerScraper.ScrapeAllSources(ctx); err != nil {
			log.Fatalf("Scraping failed: %v", err)
		}

		metricsValue := powerScraper.GetMetrics()
		metrics = &metricsValue
	}

	// Output results
	if output == "json" {
		outputJSON(metrics)
	} else {
		outputConsole(metrics)
	}
}

func runMetricsCommand(cfg *config.Config, output string) {
	// For now, we'll just show configuration as we don't have persistent metrics
	fmt.Println("Configuration-based metrics:")
	if output == "json" {
		outputJSON(cfg)
	} else {
		fmt.Printf("Concurrent Sources: %d\n", cfg.Scraper.ConcurrentSources)
		fmt.Printf("Batch Size: %d\n", cfg.Scraper.BatchSize)
		fmt.Printf("Scraping Interval: %v\n", cfg.Scraper.ScrapingInterval)
		fmt.Printf("Enabled Sources: ")

		var enabled []string
		if cfg.Sources.RemoteOK.Enabled {
			enabled = append(enabled, "RemoteOK")
		}
		if cfg.Sources.Remotive.Enabled {
			enabled = append(enabled, "Remotive")
		}
		if cfg.Sources.WeWorkRemotely.Enabled {
			enabled = append(enabled, "WeWorkRemotely")
		}

		fmt.Printf("%v\n", enabled)
	}
}

func runTestCommand(cfg *config.Config, source string, verbose bool) {
	fmt.Println("Testing job sources...")

	httpClient := httpclient.NewHttpClient(cfg.Scraper.RequestTimeout)
	logger := log.New(os.Stdout, "", log.LstdFlags)
	if !verbose {
		logger = log.New(log.Writer(), "", 0)
	}

	// Test specific source or all sources
	if source != "" {
		testSingleSource(httpClient, source, logger)
	} else {
		testAllSources(httpClient, cfg, logger)
	}
}

func runConfigCommand(cfg *config.Config, output string) {
	if output == "json" {
		outputJSON(cfg)
	} else {
		fmt.Println("Current Configuration:")
		fmt.Printf("Database URL: %s\n", maskString(cfg.Database.SupabaseURL))
		fmt.Printf("Database Key: %s\n", maskString(cfg.Database.SupabaseKey))
		fmt.Printf("Scraping Interval: %v\n", cfg.Scraper.ScrapingInterval)
		fmt.Printf("Concurrent Sources: %d\n", cfg.Scraper.ConcurrentSources)
		fmt.Printf("Monitoring Enabled: %t\n", cfg.Monitoring.Enabled)
	}
}

func runSourcesCommand(cfg *config.Config, output string) {
	sources := map[string]config.SourceConfig{
		"RemoteOK":       cfg.Sources.RemoteOK,
		"Remotive":       cfg.Sources.Remotive,
		"WeWorkRemotely": cfg.Sources.WeWorkRemotely,
	}

	if output == "json" {
		outputJSON(sources)
	} else {
		fmt.Println("Available Job Sources:")
		for name, sourceConfig := range sources {
			status := "disabled"
			if sourceConfig.Enabled {
				status = "enabled"
			}
			fmt.Printf("- %s: %s (rate limit: %d/min)\n", name, status, sourceConfig.RateLimit)
		}
	}
}

func testSingleSource(client *httpclient.HttpClient, sourceName string, logger *log.Logger) {
	fmt.Printf("Testing source: %s\n", sourceName)

	start := time.Now()

	switch sourceName {
	case "remoteok":
		source := sources.NewRemoteOKSource(client)
		jobs, err := source.FetchJobs()
		if err != nil {
			fmt.Printf("❌ RemoteOK test failed: %v\n", err)
			return
		}
		fmt.Printf("✅ RemoteOK test passed: fetched %d jobs in %v\n", len(jobs), time.Since(start))

	case "remotive":
		source := sources.NewRemotiveSource(client)
		jobs, err := source.FetchJobs()
		if err != nil {
			fmt.Printf("❌ Remotive test failed: %v\n", err)
			return
		}
		fmt.Printf("✅ Remotive test passed: fetched %d jobs in %v\n", len(jobs), time.Since(start))

	default:
		fmt.Printf("❌ Unknown source: %s\n", sourceName)
	}
}

func testAllSources(client *httpclient.HttpClient, cfg *config.Config, logger *log.Logger) {
	if cfg.Sources.RemoteOK.Enabled {
		testSingleSource(client, "remoteok", logger)
	}

	if cfg.Sources.Remotive.Enabled {
		testSingleSource(client, "remotive", logger)
	}
}

// scrapeSingleSource scrapes a specific source and returns metrics
func scrapeSingleSource(client *httpclient.HttpClient, store storage.Store, sourceName, category string, logger *log.Logger, ctx context.Context) *scraper.ScraperMetrics {
	// Initialize sources
	remoteOKSource := sources.NewRemoteOKSource(client)
	remotiveSource := sources.NewRemotiveSource(client)

	var jobs []models.Job
	var err error

	switch sourceName {
	case "remoteok":
		jobs, err = remoteOKSource.FetchJobs()
	case "remotive":
		// Check if category filtering is requested
		if category != "" {
			fmt.Printf("Fetching jobs from Remotive with category: %s\n", category)
			jobs, err = remotiveSource.FetchJobsByCategory(category)
		} else {
			jobs, err = remotiveSource.FetchJobs()
		}
	default:
		log.Fatalf("Unknown source: %s. Available sources: remoteok, remotive", sourceName)
	}

	if err != nil {
		log.Fatalf("Failed to fetch jobs from %s: %v", sourceName, err)
	}

	fmt.Printf("Fetched %d jobs from %s\n", len(jobs), sourceName)

	// Save jobs to storage
	if len(jobs) > 0 {
		if err := store.SaveJobs(jobs); err != nil {
			log.Printf("Error saving jobs to storage: %v", err)
		} else {
			fmt.Printf("Successfully saved %d jobs\n", len(jobs))
		}
	}

	// Return simplified metrics
	metrics := &scraper.ScraperMetrics{
		TotalJobsScraped: int64(len(jobs)),
		TotalJobsSaved:   int64(len(jobs)), // Assuming all are saved for now
		TotalDuplicates:  0,                // Would need actual duplicate tracking
		TotalErrors:      0,
		ScrapingDuration: time.Minute, // Approximate
	}

	return metrics
}

func outputJSON(data interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		log.Printf("Failed to encode JSON: %v", err)
	}
}

func outputConsole(metrics *scraper.ScraperMetrics) {
	fmt.Println("=== Scraping Results ===")
	fmt.Printf("Total Jobs Scraped: %d\n", metrics.TotalJobsScraped)
	fmt.Printf("Total Jobs Saved: %d\n", metrics.TotalJobsSaved)
	fmt.Printf("Total Duplicates: %d\n", metrics.TotalDuplicates)
	fmt.Printf("Total Errors: %d\n", metrics.TotalErrors)
	fmt.Printf("Scraping Duration: %v\n", metrics.ScrapingDuration)

	if len(metrics.SourcePerformance) > 0 {
		fmt.Println("\n=== Source Performance ===")
		for source, perf := range metrics.SourcePerformance {
			fmt.Printf("%s:\n", source)
			fmt.Printf("  Jobs Scraped: %d\n", perf.JobsScraped)
			fmt.Printf("  Duplicates: %d\n", perf.Duplicates)
			fmt.Printf("  Errors: %d\n", perf.Errors)
			fmt.Printf("  Response Time: %v\n", perf.ResponseTime)
		}
	}
}

func maskString(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

func printUsage() {
	fmt.Println("Job Scraper CLI Tool")
	fmt.Println("Usage:")
	fmt.Println("  scraper-cli [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  -cmd scrape    - Run job scraping")
	fmt.Println("  -cmd metrics   - Show metrics")
	fmt.Println("  -cmd test      - Test job sources")
	fmt.Println("  -cmd config    - Show configuration")
	fmt.Println("  -cmd sources   - List available sources")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -config string   - Configuration file (default: config.json)")
	fmt.Println("  -source string   - Specific source to use (remoteok, remotive)")
	fmt.Println("  -category string - Filter by category (software-dev, devops, data, etc.)")
	fmt.Println("  -output string   - Output format: console, json (default: console)")
	fmt.Println("  -verbose         - Verbose output")
	fmt.Println("  -help            - Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  scraper-cli -cmd scrape                              # Scrape all sources")
	fmt.Println("  scraper-cli -cmd scrape -source remotive             # Scrape only Remotive")
	fmt.Println("  scraper-cli -cmd scrape -source remotive -category software-dev  # Scrape software dev jobs from Remotive")
	fmt.Println("  scraper-cli -help                                    # Show help")
}
