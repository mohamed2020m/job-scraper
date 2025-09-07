package sources

import (
	"job-scraper-go/internal/models"
)

// JobSource represents a job board source
type JobSource interface {
	GetName() string
	FetchJobs() ([]models.Job, error)
	GetRateLimit() int // requests per minute
	SupportsSearch() bool
	GetBaseURL() string
}

// JobSourceConfig holds configuration for job sources
type JobSourceConfig struct {
	Enabled     bool                   `json:"enabled"`
	RateLimit   int                    `json:"rate_limit"`
	SearchTerms []string               `json:"search_terms"`
	Locations   []string               `json:"locations"`
	JobTypes    []string               `json:"job_types"`
	Custom      map[string]interface{} `json:"custom"`
}

// SourceManager manages all job sources
type SourceManager struct {
	sources map[string]JobSource
	configs map[string]JobSourceConfig
}

// NewSourceManager creates a new source manager
func NewSourceManager() *SourceManager {
	return &SourceManager{
		sources: make(map[string]JobSource),
		configs: make(map[string]JobSourceConfig),
	}
}

// RegisterSource registers a new job source
func (sm *SourceManager) RegisterSource(source JobSource, config JobSourceConfig) {
	sm.sources[source.GetName()] = source
	sm.configs[source.GetName()] = config
}

// GetSources returns all registered sources
func (sm *SourceManager) GetSources() map[string]JobSource {
	return sm.sources
}

// GetEnabledSources returns only enabled sources
func (sm *SourceManager) GetEnabledSources() map[string]JobSource {
	enabled := make(map[string]JobSource)
	for name, source := range sm.sources {
		if config, exists := sm.configs[name]; exists && config.Enabled {
			enabled[name] = source
		}
	}
	return enabled
}

// GetSourceConfig returns configuration for a source
func (sm *SourceManager) GetSourceConfig(name string) (JobSourceConfig, bool) {
	config, exists := sm.configs[name]
	return config, exists
}
