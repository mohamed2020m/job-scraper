DROP TABLE IF EXISTS jobs;

CREATE TABLE jobs (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    company TEXT NOT NULL,
    location TEXT,
    url TEXT,
    description TEXT,
    salary TEXT,
    posted_date TIMESTAMP WITH TIME ZONE,
    source TEXT NOT NULL,
    job_category TEXT,
    job_type TEXT,
    scraped_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jobs_source ON jobs(source);
CREATE INDEX idx_jobs_category ON jobs(job_category);
CREATE INDEX idx_jobs_job_type ON jobs(job_type);
CREATE INDEX idx_jobs_posted_date ON jobs(posted_date);
CREATE INDEX idx_jobs_scraped_at ON jobs(scraped_at);
CREATE INDEX idx_jobs_company ON jobs(company);
CREATE INDEX idx_jobs_location ON jobs(location);

CREATE UNIQUE INDEX idx_jobs_unique ON jobs(title, company, url) WHERE url IS NOT NULL;