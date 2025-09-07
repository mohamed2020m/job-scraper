# 🚀 Powerful Job Scraper

A robust, concurrent job scraper built in Go that extracts job listings from multiple sources with advanced features like rate limiting, deduplication, retry logic, and comprehensive monitoring.

## ✨ Features

### 🎯 **Powerful Scraping Engine**
- **Multi-source support**: RemoteOK, Remotive with extensible architecture
- **Enhanced job model**: Supports description, salary, job type, and category fields
- **Concurrent processing**: Scrape multiple sources simultaneously with intelligent rate limiting
- **Smart categorization**: Intelligent job categorization based on titles and tags
- **Retry logic**: Exponential backoff for failed requests with circuit breaker pattern
- **Deduplication**: Smart job duplicate detection using content hashing
- **Batch processing**: Efficient job saving with consistent JSON structures

### 🛡️ **Robustness & Reliability**
- **Graceful shutdown**: Clean termination with signal handling
- **Context-aware**: Proper context cancellation throughout the application
- **Error handling**: Comprehensive error tracking and reporting with fallback mechanisms  
- **Flexible date parsing**: Multiple date format support with intelligent fallbacks
- **Configurable timeouts**: Request and operation timeouts with proper resource management

### 📊 **Basic Metrics & Logging**
- **Runtime metrics**: Track jobs scraped, saved, duplicates, and errors during scraping operations
- **Configuration display**: View current scraper settings and enabled sources
- **Console logging**: Structured logging with configurable output levels
- **JSON/Console output**: Multiple output formats for basic configuration and results
- **Category filtering**: Filter jobs by specific categories (software-dev, devops, data, etc.)

### 🔧 **Configuration Management**
- **Flexible CLI**: Support for specific source scraping and category filtering
- **Environment variables**: Secure credential management with .env support
- **Runtime validation**: Configuration validation on startup with sensible defaults
- **Extensible architecture**: Easy to add new job sources with consistent interfaces

## 📁 Project Structure

```
job-scraper-go/
├── cmd/
│   ├── scraper/          # Main daemon application
│   └── scraper-cli/      # CLI tool for testing and one-off runs
├── internal/
│   ├── config/           # Configuration management
│   ├── models/           # Data models
│   ├── scraper/          # Core scraping logic
│   │   └── sources/      # Job source implementations
│   └── storage/          # Storage abstraction layer
├── pkg/
│   └── httpclient/       # HTTP client wrapper
├── config.json          # Configuration file
├── .env                  # Environment variables
└── README.md
```

## 🚀 Quick Start

### Prerequisites
- Go 1.22 or higher
- Supabase account and credentials

### Installation

1. **Clone the repository**
```bash
git clone <repository-url>
cd job-scraper-go
```

2. **Set up environment**
```bash
cp .env.example .env
# Edit .env with your Supabase credentials
```

3. **Configure the scraper**
```bash
# config.json contains all scraper settings
# Modify sources, rate limits, intervals as needed
```

4. **Build the applications**
```bash
go build ./cmd/scraper
go build ./cmd/scraper-cli
```

### Usage

#### 🤖 **Daemon Mode** (Continuous scraping)
```bash
# Run with default config
./scraper

# Run with custom config
./scraper -config custom-config.json
```

#### 🛠️ **CLI Mode** (One-off operations)
```bash
# Show help and available options
./scraper-cli -help

# Test all sources
./scraper-cli -cmd test

# Test specific source
./scraper-cli -cmd test -source remoteok
./scraper-cli -cmd test -source remotive

# Scrape all sources
./scraper-cli -cmd scrape

# Scrape specific source
./scraper-cli -cmd scrape -source remoteok
./scraper-cli -cmd scrape -source remotive

# Scrape with category filtering (Remotive only)
./scraper-cli -cmd scrape -source remotive -category software-dev
./scraper-cli -cmd scrape -source remotive -category devops

# Show configuration
./scraper-cli -cmd config

# List available sources
./scraper-cli -cmd sources

# Show configuration (current metrics command)
./scraper-cli -cmd metrics -output json
```

## ⚙️ Configuration

### Main Configuration (`config.json`)
```json
{
  "scraper": {
    "concurrent_sources": 5,        // Max concurrent sources
    "batch_size": 50,               // Jobs per batch save
    "retry_attempts": 3,            // Max retry attempts
    "scraping_interval": 900000000000, // 15 minutes in nanoseconds
    "request_timeout": 30000000000     // 30 seconds
  },
  "sources": {
    "remoteok": {
      "enabled": true,
      "rate_limit": 60,             // Requests per minute
      "search_terms": ["golang", "go", "backend"]
    }
  }
}
```

### Environment Variables (`.env`)
```bash
SUPABASE_URL=your_supabase_url
SUPABASE_KEY=your_supabase_key
```

## 🗃️ Database Schema

The scraper uses an enhanced job table structure in Supabase:

```sql
CREATE TABLE jobs (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    company TEXT NOT NULL,
    location TEXT,
    url TEXT,
    description TEXT,           -- Job description from source
    salary TEXT,               -- Salary information when available
    posted_date TIMESTAMP WITH TIME ZONE,  -- Original posting date
    source TEXT NOT NULL,      -- Source name (RemoteOK, Remotive)
    job_category TEXT,         -- Categorized job type
    job_type TEXT,            -- Employment type (full-time, contract, etc.)
    scraped_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Performance indexes
CREATE INDEX idx_jobs_source ON jobs(source);
CREATE INDEX idx_jobs_category ON jobs(job_category);  
CREATE INDEX idx_jobs_job_type ON jobs(job_type);
CREATE INDEX idx_jobs_posted_date ON jobs(posted_date);
CREATE INDEX idx_jobs_scraped_at ON jobs(scraped_at);

-- Prevent duplicates
CREATE UNIQUE INDEX idx_jobs_unique ON jobs(title, company, url) 
WHERE url IS NOT NULL;
```

### Field Descriptions
- **description**: Full job description when available from source
- **salary**: Salary information (mainly from Remotive)
- **job_category**: Intelligent categorization (Backend, Frontend, DevOps, etc.)
- **job_type**: Employment type (full-time, part-time, contract, freelance)

## 🔌 Extending the Scraper

### Adding a New Job Source

1. **Implement the JobSource interface**:
```go
type JobSource interface {
    GetName() string
    FetchJobs() ([]models.Job, error)
    GetRateLimit() int // requests per minute
    SupportsSearch() bool
    GetBaseURL() string
}
```

2. **Create your source struct**:
```go
type MyJobSource struct {
    client  *httpclient.HttpClient
    baseURL string
}

func (m *MyJobSource) FetchJobs() ([]models.Job, error) {
    // Ensure consistent field population to avoid batch errors
    job := models.Job{
        Title:       title,
        Company:     company,
        Location:    location,
        URL:         url,
        Description: description, // Use " " if empty to maintain consistency
        Salary:      salary,      // Use " " if empty to maintain consistency
        PostedDate:  postedDate,
        Source:      m.GetName(),
        JobCategory: category,    // Use " " if empty to maintain consistency
        JobType:     jobType,
    }
    return jobs, nil
}
```

3. **Register in PowerScraper**:
```go
func (ps *PowerScraper) InitializeSources() {
    mySource := sources.NewMyJobSource(ps.client)
    ps.sourceManager.RegisterSource(mySource, sources.JobSourceConfig{
        Enabled:   true,
        RateLimit: mySource.GetRateLimit(),
    })
}
```

### Important Notes
- **Consistent JSON structures**: Ensure all jobs have the same fields to avoid batch insert errors
- **Date parsing**: Support multiple date formats with fallback mechanisms
- **Error handling**: Implement robust error handling with retries
- **Rate limiting**: Respect API limits to avoid being blocked

## 🏗️ Architecture Patterns

### Rate Limiting
- **Token bucket algorithm** per source
- **Concurrent-safe** with mutex protection
- **Dynamic rate limit** adjustment

### Deduplication
- **Content-based hashing** using MD5
- **Jaccard similarity** for fuzzy matching
- **Thread-safe** operations

### Error Handling
- **Exponential backoff** with jitter
- **Circuit breaker** pattern for failing sources
- **Graceful degradation**

### Concurrency
- **Worker pool pattern** for sources
- **Semaphore-based** concurrency limiting
- **Context cancellation** throughout

## Basic Metrics

The scraper provides basic metrics during scraping operations:

```
=== Runtime Metrics (During Scraping) ===
Total Jobs Scraped: 1555
Total Jobs Saved: 1555  
Total Duplicates: 0
Total Errors: 0
Scraping Duration: 45.2s

=== Source Performance ===
RemoteOK: scraped=97, saved=97, duplicates=0, errors=0, response_time=393ms
Remotive: scraped=1458, saved=1458, duplicates=0, errors=0, response_time=2.1s
```

**Note**: Metrics are displayed during scraping operations but are not persisted between runs.

### Available Commands
- `./scraper-cli -cmd metrics` - Show configuration settings (no persistent metrics yet)
- `./scraper-cli -cmd test` - Test all sources connectivity
- `./scraper-cli -cmd sources` - List available sources and status

## 🛠️ Development

### Testing
```bash
# Test all sources
go run ./cmd/scraper-cli -cmd test

# Test with verbose output
go run ./cmd/scraper-cli -cmd test -verbose

# Run unit tests
go test ./...
```

### Building
```bash
# Build all binaries
go build ./...

# Build specific binary
go build -o scraper ./cmd/scraper
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License. See the LICENSE file for more details.

---

**Built with Go 1.22** • **Powered by Supabase** • **Production Ready**