package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"udemy-course-notifier/security"
)

type Config struct {
	Telegram struct {
		Token     string `yaml:"token"`
		ChannelID string `yaml:"channel_id"`
	} `yaml:"telegram"`
	
	Scraping struct {
		IntervalMinutes      int      `yaml:"interval_minutes"`
		SourceURLs          []string `yaml:"source_urls"`
		UserAgent           string   `yaml:"user_agent"`
		RateLimitDelaySeconds int    `yaml:"rate_limit_delay_seconds"`
	} `yaml:"scraping"`
	
	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`
	
	Filters struct {
		DefaultCategories   []string `yaml:"default_categories"`
		MinRating          float64  `yaml:"min_rating"`
		MaxCoursesPerHour  int      `yaml:"max_courses_per_hour"`
	} `yaml:"filters"`
	
	Logging struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"logging"`
}

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables if set
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		config.Telegram.Token = token
	}
	
	if channelID := os.Getenv("TELEGRAM_CHANNEL_ID"); channelID != "" {
		config.Telegram.ChannelID = channelID
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

func (c *Config) validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram token is required")
	}
	
	if c.Telegram.ChannelID == "" {
		return fmt.Errorf("telegram channel ID is required")
	}

	// Validate channel ID format
	if err := security.ValidateChannelID(c.Telegram.ChannelID); err != nil {
		return fmt.Errorf("invalid channel ID: %w", err)
	}
	
	if len(c.Scraping.SourceURLs) == 0 {
		return fmt.Errorf("at least one source URL is required")
	}

	// Validate all source URLs
	for _, url := range c.Scraping.SourceURLs {
		if err := security.ValidateURL(url); err != nil {
			return fmt.Errorf("invalid source URL %s: %w", url, err)
		}
	}

	// Validate file paths
	if err := security.ValidateFilePath(c.Database.Path); err != nil {
		return fmt.Errorf("invalid database path: %w", err)
	}

	if c.Logging.File != "" {
		if err := security.ValidateFilePath(c.Logging.File); err != nil {
			return fmt.Errorf("invalid log file path: %w", err)
		}
	}
	
	return nil
}