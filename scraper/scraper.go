package scraper

import (
	"fmt"
	"log"
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

func (s *Scraper) extractCourses(doc *goquery.Document, sourceURL string) ([]database.Course, error) {
	var courses []database.Course
	count := 0
	
	// This is a generic scraper - specific sites may need custom selectors
	// Look for both direct Udemy links and coupon page links
	log.Printf("Scanning %s for course links...", sourceURL)
	doc.Find("a[href*='udemy.com'], a[href*='/coupon/']").Each(func(i int, selection *goquery.Selection) {
		if count >= security.LimitCourses(1000) {
			return // Stop processing if we hit the limit
		}

		href, exists := selection.Attr("href")
		if !exists {
			return
		}

		var courseURL string
		var err error

		// Handle coupon page links vs direct Udemy links
		if strings.Contains(href, "/coupon/") {
			// This is a coupon page link, follow it to get the Udemy URL
			fullURL := href
			if strings.HasPrefix(href, "/") {
				parsedSourceURL, _ := url.Parse(sourceURL)
				fullURL = parsedSourceURL.Scheme + "://" + parsedSourceURL.Host + href
			}
			
			courseURL, err = s.followCouponLink(fullURL)
			if err != nil {
				log.Printf("Failed to follow coupon link %s: %v", fullURL, err)
				return // Skip if we can't get the Udemy URL
			}
		} else {
			// Validate URL before processing
			if err := security.ValidateURL(href); err != nil {
				return // Skip invalid URLs
			}

			// Clean and validate URL
			courseURL, err = s.cleanUdemyURL(href)
			if err != nil {
				return
			}
		}

		title := strings.TrimSpace(selection.Text())
		if title == "" {
			// Try to find title in parent elements
			title = strings.TrimSpace(selection.Parent().Text())
		}

		if title == "" || len(title) < 10 { // Skip if no meaningful title
			return
		}

		// Sanitize and validate title length
		title = security.SanitizeString(title)
		if len(title) > 200 { // Reasonable title length limit
			title = title[:200]
		}

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

	// If it's a tracking URL (like linksynergy), preserve it completely
	if strings.Contains(parsedURL.Host, "linksynergy.com") || 
	   strings.Contains(parsedURL.Host, "click.") ||
	   strings.Contains(rawURL, "murl=") {
		return rawURL, nil // Keep tracking URLs intact
	}

	// Ensure it's a Udemy URL - but allow tracking domains
	if !strings.Contains(parsedURL.Host, "udemy.com") && !strings.Contains(rawURL, "udemy.com") {
		return "", fmt.Errorf("not a Udemy URL: %s", rawURL)
	}

	// For direct Udemy URLs, clean query parameters but keep coupon codes
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

func (s *Scraper) followCouponLink(couponURL string) (string, error) {
	time.Sleep(s.rateLimit) // Rate limiting
	
	req, err := http.NewRequest("GET", couponURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch coupon page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("coupon page returned status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse coupon page HTML: %w", err)
	}

	// Look for Udemy course links on the coupon page (not user profiles)
	var udemyURL string
	var allUdemyLinks []string
	
	doc.Find("a[href*='udemy.com']").Each(func(i int, selection *goquery.Selection) {
		href, exists := selection.Attr("href")
		if exists {
			allUdemyLinks = append(allUdemyLinks, href)
			if udemyURL == "" && strings.Contains(href, "/course/") {
				udemyURL = href
			}
		}
	})
	
	// If no direct course link found, take the first non-user link
	if udemyURL == "" {
		for _, link := range allUdemyLinks {
			if !strings.Contains(link, "/user/") {
				udemyURL = link
				break
			}
		}
	}

	// If no Udemy link found, look for claim button
	if udemyURL == "" {
		var claimURL string
		doc.Find("a[href*='/claim/']").Each(func(i int, selection *goquery.Selection) {
			if claimURL == "" {
				href, exists := selection.Attr("href")
				if exists {
					claimURL = href
				}
			}
		})
		
		if claimURL != "" {
			// Follow the claim link to get the actual Udemy URL
			fullClaimURL := claimURL
			if strings.HasPrefix(claimURL, "/") {
				parsedCouponURL, _ := url.Parse(couponURL)
				fullClaimURL = parsedCouponURL.Scheme + "://" + parsedCouponURL.Host + claimURL
			}
			
			udemyURL, err = s.followClaimLink(fullClaimURL)
			if err != nil {
				log.Printf("Failed to follow claim link %s: %v", fullClaimURL, err)
				return "", fmt.Errorf("failed to follow claim link: %w", err)
			}
		}
	}

	if udemyURL == "" {
		return "", fmt.Errorf("no Udemy link found on coupon page")
	}

	return s.cleanUdemyURL(udemyURL)
}

func (s *Scraper) followClaimLink(claimURL string) (string, error) {
	time.Sleep(s.rateLimit) // Rate limiting
	
	req, err := http.NewRequest("GET", claimURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch claim page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("claim page returned status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse claim page HTML: %w", err)
	}

	// Look for Udemy course links on the claim page
	var udemyURL string
	var allLinks []string
	
	// Collect all links that might be Udemy-related
	doc.Find("a").Each(func(i int, selection *goquery.Selection) {
		href, exists := selection.Attr("href")
		if exists {
			allLinks = append(allLinks, href)
			if strings.Contains(href, "udemy.com") && strings.Contains(href, "/course/") {
				udemyURL = href
			}
		}
	})
	
	log.Printf("Found %d total links on claim page", len(allLinks))
	
	// If no course link found, take any Udemy link that's not a user profile
	if udemyURL == "" {
		for _, href := range allLinks {
			if strings.Contains(href, "udemy.com") && !strings.Contains(href, "/user/") {
				udemyURL = href
				break
			}
		}
	}

	if udemyURL == "" {
		return "", fmt.Errorf("no Udemy course link found on claim page")
	}

	return udemyURL, nil
}