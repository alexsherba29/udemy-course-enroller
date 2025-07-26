package scraper

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"udemy-course-notifier/database"
	"udemy-course-notifier/security"
)

type Scraper struct {
	client    *http.Client
	userAgent string
	rateLimit time.Duration
}

func New(userAgent string, rateLimitSeconds int) *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: userAgent,
		rateLimit: time.Duration(rateLimitSeconds) * time.Second,
	}
}

func (s *Scraper) ScrapeCoursesFromURL(sourceURL string) ([]database.Course, error) {
	time.Sleep(s.rateLimit) // Rate limiting

	req, err := http.NewRequest("GET", sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("received status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return s.extractCourses(doc, sourceURL)
}

func (s *Scraper) extractCourses(doc *goquery.Document, _ string) ([]database.Course, error) {
	var courses []database.Course
	count := 0
	
	// This is a generic scraper - specific sites may need custom selectors
	doc.Find("a[href*='udemy.com']").Each(func(i int, selection *goquery.Selection) {
		if count >= security.LimitCourses(1000) {
			return // Stop processing if we hit the limit
		}

		href, exists := selection.Attr("href")
		if !exists {
			return
		}

		// Validate URL before processing
		if err := security.ValidateURL(href); err != nil {
			return // Skip invalid URLs
		}

		// Clean and validate URL
		courseURL, err := s.cleanUdemyURL(href)
		if err != nil {
			return
		}

		title := strings.TrimSpace(selection.Text())
		if title == "" {
			// Try to find title in parent elements
			title = strings.TrimSpace(selection.Parent().Text())
		}

		if title == "" || len(title) < 10 { // Skip if no meaningful title
			return
		}

		// Sanitize title
		title = security.SanitizeString(title)

		// Extract basic course info
		course := database.Course{
			URL:         courseURL,
			Title:       title,
			Description: security.SanitizeString(s.extractDescription(selection)),
			Category:    security.SanitizeString(s.extractCategory(selection)),
			Rating:      s.extractRating(selection),
			Price:       security.SanitizeString(s.extractPrice(selection)),
			Discount:    "Free", // Assuming we're looking for free courses
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour), // Default 7 days
		}

		courses = append(courses, course)
		count++
	})

	return courses, nil
}

func (s *Scraper) cleanUdemyURL(rawURL string) (string, error) {
	// Handle relative URLs
	if strings.HasPrefix(rawURL, "/") {
		rawURL = "https://www.udemy.com" + rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Ensure it's a Udemy URL
	if !strings.Contains(parsedURL.Host, "udemy.com") {
		return "", fmt.Errorf("not a Udemy URL")
	}

	// Clean query parameters but keep coupon codes
	query := parsedURL.Query()
	cleanQuery := url.Values{}
	
	// Keep important parameters
	if coupon := query.Get("couponCode"); coupon != "" {
		cleanQuery.Set("couponCode", coupon)
	}
	if ref := query.Get("referralCode"); ref != "" {
		cleanQuery.Set("referralCode", ref)
	}

	parsedURL.RawQuery = cleanQuery.Encode()
	return parsedURL.String(), nil
}

func (s *Scraper) extractDescription(selection *goquery.Selection) string {
	// Look for description in common places
	desc := selection.AttrOr("title", "")
	if desc == "" {
		desc = selection.Parent().Find(".description, .course-description").First().Text()
	}
	return strings.TrimSpace(desc)
}

func (s *Scraper) extractCategory(selection *goquery.Selection) string {
	// Look for category information
	category := selection.Parent().Find(".category, .course-category").First().Text()
	if category == "" {
		category = "General"
	}
	return strings.TrimSpace(category)
}

func (s *Scraper) extractRating(selection *goquery.Selection) float64 {
	// Look for rating information
	ratingText := selection.Parent().Find(".rating, .course-rating").First().Text()
	
	// Extract number from text like "4.5 stars" or "Rating: 4.2"
	re := regexp.MustCompile(`(\d+\.?\d*)`)
	matches := re.FindStringSubmatch(ratingText)
	
	if len(matches) > 1 {
		if rating, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return rating
		}
	}
	
	return 0.0
}

func (s *Scraper) extractPrice(selection *goquery.Selection) string {
	// Look for price information
	price := selection.Parent().Find(".price, .course-price").First().Text()
	if strings.Contains(strings.ToLower(price), "free") {
		return "Free"
	}
	
	// Extract price like "$99.99" or "€29.99"
	re := regexp.MustCompile(`[£$€¥₹]\d+(?:\.\d{2})?`)
	if match := re.FindString(price); match != "" {
		return match
	}
	
	return "Unknown"
}