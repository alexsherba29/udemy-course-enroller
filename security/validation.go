package security

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	MaxMessageLength     = 4096  // Telegram message limit
	MaxFilterStringLength = 1000
	MaxCourseCount       = 100   // Max courses to process per scrape
)

var (
	allowedDomains = []string{
		"udemy.com",
		"www.udemy.com",
		"courson.xyz",
	}
	
	// Regex for basic input sanitization
	safeStringRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-_.,|:]+$`)
)

// ValidateURL ensures URL is safe and points to allowed domains
func ValidateURL(rawURL string) error {
	if len(rawURL) > 2048 {
		return fmt.Errorf("URL too long")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check protocol
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("invalid URL scheme: %s", parsedURL.Scheme)
	}

	// Check domain allowlist
	host := strings.ToLower(parsedURL.Host)
	allowed := false
	for _, domain := range allowedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("domain not allowed: %s", host)
	}

	return nil
}

// ValidateFilePath ensures file path is safe
func ValidateFilePath(path string) error {
	if len(path) > 255 {
		return fmt.Errorf("file path too long")
	}

	// Prevent path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected")
	}

	// Clean the path
	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		return fmt.Errorf("path contains dangerous elements")
	}

	return nil
}

// SanitizeString removes dangerous characters from user input
func SanitizeString(input string) string {
	if len(input) > MaxFilterStringLength {
		input = input[:MaxFilterStringLength]
	}

	// Remove potentially dangerous characters
	input = strings.ReplaceAll(input, "\x00", "")
	input = strings.ReplaceAll(input, "\n", " ")
	input = strings.ReplaceAll(input, "\r", " ")
	input = strings.ReplaceAll(input, "\t", " ")

	return strings.TrimSpace(input)
}

// ValidateFilterString validates user filter input
func ValidateFilterString(filter string) error {
	if len(filter) > MaxFilterStringLength {
		return fmt.Errorf("filter string too long")
	}

	sanitized := SanitizeString(filter)
	if len(sanitized) == 0 {
		return fmt.Errorf("filter string is empty after sanitization")
	}

	return nil
}

// ValidateChannelID validates Telegram channel ID format
func ValidateChannelID(channelID string) error {
	if len(channelID) == 0 {
		return fmt.Errorf("channel ID cannot be empty")
	}

	// Basic validation for Telegram channel ID format
	// Allow @username, -chatid, or numeric user IDs
	if !strings.HasPrefix(channelID, "@") && !strings.HasPrefix(channelID, "-") {
		// Check if it's a numeric user ID
		if _, err := strconv.ParseInt(channelID, 10, 64); err != nil {
			return fmt.Errorf("invalid channel ID format")
		}
	}

	return nil
}

// LimitCourses limits the number of courses to prevent memory exhaustion
func LimitCourses(count int) int {
	if count > MaxCourseCount {
		return MaxCourseCount
	}
	return count
}