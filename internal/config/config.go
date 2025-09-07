package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Config holds the application configuration
type Config struct {
	Server     ServerConfig     `json:"server"`
	Database   DatabaseConfig   `json:"database"`
	Scraper    ScraperConfig    `json:"scraper"`
	Sources    SourcesConfig    `json:"sources"`
	Monitoring MonitoringConfig `json:"monitoring"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port         int           `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
	IdleTimeout  time.Duration `json:"idle_timeout"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	SupabaseURL string `json:"supabase_url"`
	SupabaseKey string `json:"supabase_key"`
}

// ScraperConfig holds scraper configuration
type ScraperConfig struct {
	ConcurrentSources int           `json:"concurrent_sources"`
	BatchSize         int           `json:"batch_size"`
	RetryAttempts     int           `json:"retry_attempts"`
	RetryDelay        time.Duration `json:"retry_delay"`
	ScrapingInterval  time.Duration `json:"scraping_interval"`
	RequestTimeout    time.Duration `json:"request_timeout"`
	EnableDedup       bool          `json:"enable_dedup"`
}

// SourcesConfig holds configuration for all job sources
type SourcesConfig struct {
	RemoteOK       SourceConfig `json:"remoteok"`
	Remotive       SourceConfig `json:"remotive"`
	WeWorkRemotely SourceConfig `json:"wework_remotely"`
}

// SourceConfig holds configuration for individual sources
type SourceConfig struct {
	Enabled     bool     `json:"enabled"`
	RateLimit   int      `json:"rate_limit"`
	SearchTerms []string `json:"search_terms"`
	Locations   []string `json:"locations"`
	JobTypes    []string `json:"job_types"`
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	Enabled         bool          `json:"enabled"`
	MetricsInterval time.Duration `json:"metrics_interval"`
	LogLevel        string        `json:"log_level"`
	LogFile         string        `json:"log_file"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Database: DatabaseConfig{
			SupabaseURL: os.Getenv("SUPABASE_URL"),
			SupabaseKey: os.Getenv("SUPABASE_KEY"),
		},
		Scraper: ScraperConfig{
			ConcurrentSources: 5,
			BatchSize:         50,
			RetryAttempts:     3,
			RetryDelay:        2 * time.Second,
			ScrapingInterval:  15 * time.Minute,
			RequestTimeout:    30 * time.Second,
			EnableDedup:       true,
		},
		Sources: SourcesConfig{
			RemoteOK: SourceConfig{
				Enabled:     true,
				RateLimit:   60,
				SearchTerms: []string{"golang", "go", "backend", "api", "microservices"},
				Locations:   []string{"remote", "worldwide"},
				JobTypes:    []string{"full-time", "contract"},
			},
			Remotive: SourceConfig{
				Enabled:     true,
				RateLimit:   100,
				SearchTerms: []string{"software-dev", "devops", "data"},
				Locations:   []string{"remote"},
				JobTypes:    []string{"full_time", "contract"},
			},
			WeWorkRemotely: SourceConfig{
				Enabled:     false,
				RateLimit:   30,
				SearchTerms: []string{"backend", "go", "api"},
				Locations:   []string{"remote"},
				JobTypes:    []string{"full-time"},
			},
		},
		Monitoring: MonitoringConfig{
			Enabled:         true,
			MetricsInterval: 1 * time.Minute,
			LogLevel:        "info",
			LogFile:         "logs/scraper.log",
		},
	}
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(filename string) (*Config, error) {
	// Start with default config
	config := DefaultConfig()

	// If file doesn't exist, return default config
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return config, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to a JSON file
func (c *Config) SaveConfig(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Database.SupabaseURL == "" {
		return fmt.Errorf("supabase URL is required")
	}

	if c.Database.SupabaseKey == "" {
		return fmt.Errorf("supabase key is required")
	}

	if c.Scraper.ConcurrentSources <= 0 {
		return fmt.Errorf("concurrent sources must be positive")
	}

	if c.Scraper.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}

	if c.Scraper.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}

	// Validate at least one source is enabled
	hasEnabledSource := c.Sources.RemoteOK.Enabled ||
		c.Sources.Remotive.Enabled ||
		c.Sources.WeWorkRemotely.Enabled

	if !hasEnabledSource {
		return fmt.Errorf("at least one job source must be enabled")
	}

	return nil
}
