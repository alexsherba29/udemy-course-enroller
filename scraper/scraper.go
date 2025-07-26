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
		rating := s.extractRating(selection)
		studentCount := s.extractStudentCount(selection)
		description := security.SanitizeString(s.extractDescription(selection))
		price := security.SanitizeString(s.extractPrice(selection))
		discount := s.extractDiscount(selection, price)
		
		course := database.Course{
			URL:          courseURL,
			Title:        title,
			Description:  description,
			Category:     security.SanitizeString(s.extractCategory(selection)),
			Rating:       rating,
			Price:        price,
			Discount:     discount,
			ExpiresAt:    s.extractExpirationDate(courseURL, title),
			StudentCount: studentCount,
			QualityScore: s.calculateQualityScore(rating, studentCount, title, description),
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
	// Look for category information in various places
	var category string
	
	// Try explicit category selectors first
	category = selection.Parent().Find(".category, .course-category, .breadcrumb, .tag").First().Text()
	
	// If no category found, try to extract from course URL
	if category == "" {
		href, exists := selection.Attr("href")
		if exists {
			category = s.extractCategoryFromURL(href)
		}
	}
	
	// If still no category, try to infer from title
	if category == "" {
		title := strings.ToLower(selection.Text())
		category = s.inferCategoryFromTitle(title)
	}
	
	// Default fallback
	if category == "" {
		category = "General"
	}
	
	return strings.TrimSpace(category)
}

func (s *Scraper) extractCategoryFromURL(url string) string {
	// Extract category from Udemy URL structure
	// Example: /course/python-programming/ -> Programming
	if strings.Contains(url, "/course/") {
		parts := strings.Split(url, "/course/")
		if len(parts) > 1 {
			coursePath := strings.Split(parts[1], "/")[0]
			return s.beautifyCategory(coursePath)
		}
	}
	return ""
}

func (s *Scraper) inferCategoryFromTitle(title string) string {
	// Category keywords mapping
	categoryMap := map[string]string{
		"python":      "Programming",
		"javascript":  "Programming", 
		"java":        "Programming",
		"golang":      "Programming",
		"react":       "Web Development",
		"angular":     "Web Development",
		"vue":         "Web Development",
		"html":        "Web Development",
		"css":         "Web Development",
		"data":        "Data Science",
		"analytics":   "Data Science",
		"machine":     "Data Science",
		"ai":          "Artificial Intelligence",
		"design":      "Design",
		"photoshop":   "Design",
		"marketing":   "Marketing",
		"business":    "Business",
		"excel":       "Business",
		"photography": "Photography",
		"music":       "Music",
		"fitness":     "Health & Fitness",
		"yoga":        "Health & Fitness",
		"language":    "Language",
		"english":     "Language",
		"spanish":     "Language",
		"finance":     "Finance",
		"investing":   "Finance",
		"crypto":      "Finance",
	}
	
	for keyword, category := range categoryMap {
		if strings.Contains(title, keyword) {
			return category
		}
	}
	
	return ""
}

func (s *Scraper) beautifyCategory(category string) string {
	// Convert URL-style categories to readable format
	category = strings.ReplaceAll(category, "-", " ")
	category = strings.ReplaceAll(category, "_", " ")
	
	// Capitalize words
	words := strings.Fields(category)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	
	return strings.Join(words, " ")
}

func (s *Scraper) extractRating(selection *goquery.Selection) float64 {
	// The selection is the link element, we need to look for rating in the course info
	// First try to find the rating in the current element or its closest siblings
	
	// Try to find rating in the same container as the link
	var targetText string
	
	// Look in the immediate parent/container
	container := selection.Closest("div, article, section")
	if container.Length() > 0 {
		targetText = container.Text()
	} else {
		// Fallback to parent
		targetText = selection.Parent().Text()
	}
	
	maxLen := 100
	if len(targetText) < maxLen {
		maxLen = len(targetText)
	}
	// DEBUG: log.Printf("DEBUG: Extracting rating from container text: %s", targetText[:maxLen])
	
	// Look for the specific course title to find the right rating
	title := strings.TrimSpace(selection.Text())
	if title != "" {
		// Find the position of the current course title in the text
		titleIndex := strings.Index(targetText, title)
		if titleIndex >= 0 {
			// Extract text around the title (next 200 chars)
			endIndex := titleIndex + len(title) + 200
			if endIndex > len(targetText) {
				endIndex = len(targetText)
			}
			courseText := targetText[titleIndex:endIndex]
			
			// Look for rating pattern in this specific course section
			re := regexp.MustCompile(`(\d+\.\d+)\s*\(\d+\s+students?\)`)
			matches := re.FindStringSubmatch(courseText)
			
			if len(matches) > 1 {
				if rating, err := strconv.ParseFloat(matches[1], 64); err == nil && rating > 0 && rating <= 5 {
					// DEBUG: log.Printf("DEBUG: Found rating: %f for course: %s", rating, title[:50])
					return rating
				}
			}
		}
	}
	
	// DEBUG: log.Printf("DEBUG: No rating found for course: %s", title[:50])
	return 0.0
}

func (s *Scraper) extractPrice(selection *goquery.Selection) string {
	// First check if this is a free course from coupon code
	href, exists := selection.Attr("href")
	if exists && (strings.Contains(href, "couponCode=") || strings.Contains(href, "/coupon/")) {
		return "Free (Coupon)"
	}
	
	// Look for price information in various selectors
	var priceText string
	
	// Try multiple selectors for price
	priceSelectors := []string{
		".price", ".course-price", ".original-price", ".current-price", 
		".price-text", "[data-price]", ".cost", ".fee",
	}
	
	container := selection.Closest("div, article, section")
	for _, selector := range priceSelectors {
		if price := container.Find(selector).First().Text(); price != "" {
			priceText = price
			break
		}
	}
	
	// If no price found in container, check parent
	if priceText == "" {
		for _, selector := range priceSelectors {
			if price := selection.Parent().Find(selector).First().Text(); price != "" {
				priceText = price
				break
			}
		}
	}
	
	// Check for free indicators
	priceTextLower := strings.ToLower(priceText)
	if strings.Contains(priceTextLower, "free") || 
	   strings.Contains(priceTextLower, "gratis") ||
	   strings.Contains(priceTextLower, "gratuito") ||
	   priceTextLower == "0" || priceTextLower == "$0" {
		return "Free"
	}
	
	// Extract price with currency symbols
	priceRegex := regexp.MustCompile(`([£$€¥₹₱₩₪₫₡₦₨₴₵₷₸₺₼₽¢]\s*\d+(?:[.,]\d{2})?|\d+(?:[.,]\d{2})?\s*[£$€¥₹₱₩₪₫₡₦₨₴₵₷₸₺₼₽])`)
	if match := priceRegex.FindString(priceText); match != "" {
		return strings.TrimSpace(match)
	}
	
	// Look for numeric price patterns
	numericRegex := regexp.MustCompile(`\d+(?:[.,]\d{2})?`)
	if match := numericRegex.FindString(priceText); match != "" && match != "0" {
		return "$" + match // Default to USD if no currency symbol
	}
	
	// If we found price text but couldn't extract a price, return it as-is
	if priceText != "" {
		return strings.TrimSpace(priceText)
	}
	
	// Default to Free for courses found on coupon sites
	return "Free"
}

func (s *Scraper) extractDiscount(selection *goquery.Selection, price string) string {
	// If price indicates it's free, this is a discount
	if strings.Contains(strings.ToLower(price), "free") || 
	   strings.Contains(strings.ToLower(price), "coupon") {
		return "100%"
	}
	
	// Look for discount indicators
	container := selection.Closest("div, article, section")
	discountSelectors := []string{
		".discount", ".sale", ".offer", ".percent-off", ".savings",
		".original-price", ".was-price", ".strike", ".strikethrough",
	}
	
	for _, selector := range discountSelectors {
		if discountText := container.Find(selector).First().Text(); discountText != "" {
			// Extract percentage discounts
			percentRegex := regexp.MustCompile(`(\d+)%`)
			if match := percentRegex.FindString(discountText); match != "" {
				return match
			}
			
			// Look for "was $X now free" patterns
			if strings.Contains(strings.ToLower(discountText), "free") {
				return "100%"
			}
		}
	}
	
	// Check URL for coupon codes (indicates free/discounted)
	href, exists := selection.Attr("href")
	if exists && (strings.Contains(href, "couponCode=") || strings.Contains(href, "/coupon/")) {
		return "100%"
	}
	
	// If we can't determine discount, assume it's available at listed price
	return "0%"
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

func (s *Scraper) extractStudentCount(selection *goquery.Selection) int {
	// Use the same approach as rating extraction to find the right course section
	var targetText string
	
	// Look in the immediate parent/container
	container := selection.Closest("div, article, section")
	if container.Length() > 0 {
		targetText = container.Text()
	} else {
		targetText = selection.Parent().Text()
	}
	
	// Look for the specific course title to find the right student count
	title := strings.TrimSpace(selection.Text())
	if title != "" {
		// Find the position of the current course title in the text
		titleIndex := strings.Index(targetText, title)
		if titleIndex >= 0 {
			// Extract text around the title (next 200 chars)
			endIndex := titleIndex + len(title) + 200
			if endIndex > len(targetText) {
				endIndex = len(targetText)
			}
			courseText := targetText[titleIndex:endIndex]
			
			// Look for student count pattern in this specific course section
			re := regexp.MustCompile(`\((\d+)\s+students?\)`)
			matches := re.FindStringSubmatch(courseText)
			
			if len(matches) > 1 {
				if count, err := strconv.Atoi(matches[1]); err == nil {
					// DEBUG: log.Printf("DEBUG: Found student count: %d for course: %s", count, title[:50])
					return count
				}
			}
		}
	}
	
	// DEBUG: log.Printf("DEBUG: No student count found for course: %s", title[:50])
	return 0
}

func (s *Scraper) extractExpirationDate(courseURL, title string) time.Time {
	// Default expiration (7 days from now)
	defaultExpiration := time.Now().Add(7 * 24 * time.Hour)
	
	// Try to extract date from coupon code in URL
	if strings.Contains(courseURL, "couponCode=") {
		// Extract coupon code
		parsedURL, err := url.Parse(courseURL)
		if err == nil {
			if murl := parsedURL.Query().Get("murl"); murl != "" {
				// Decode the murl parameter to get the actual Udemy URL
				decodedURL, err := url.QueryUnescape(murl)
				if err == nil {
					innerURL, err := url.Parse(decodedURL)
					if err == nil {
						couponCode := innerURL.Query().Get("couponCode")
						if couponCode != "" {
							if expiration := s.parseCouponExpiration(couponCode); !expiration.IsZero() {
								return expiration
							}
						}
					}
				}
			}
		}
	}
	
	// Intelligent defaults based on course characteristics
	// High-quality courses tend to have longer validity
	// Popular courses (mentioned in title) might expire faster
	if strings.Contains(strings.ToLower(title), "limited") || 
	   strings.Contains(strings.ToLower(title), "special") ||
	   strings.Contains(strings.ToLower(title), "exclusive") {
		return time.Now().Add(2 * 24 * time.Hour) // 2 days for "limited" offers
	}
	
	return defaultExpiration
}

func (s *Scraper) parseCouponExpiration(couponCode string) time.Time {
	// Extract date-like parts from coupon code
	// Look for patterns like "22JULY2025", "JULY2025", "2025", etc.
	
	// Month name patterns
	monthMap := map[string]time.Month{
		"JAN": time.January, "JANUARY": time.January,
		"FEB": time.February, "FEBRUARY": time.February,
		"MAR": time.March, "MARCH": time.March,
		"APR": time.April, "APRIL": time.April,
		"MAY": time.May,
		"JUN": time.June, "JUNE": time.June,
		"JUL": time.July, "JULY": time.July,
		"AUG": time.August, "AUGUST": time.August,
		"SEP": time.September, "SEPTEMBER": time.September,
		"OCT": time.October, "OCTOBER": time.October,
		"NOV": time.November, "NOVEMBER": time.November,
		"DEC": time.December, "DECEMBER": time.December,
	}
	
	// Check for month name patterns like "22JULY2025"
	for monthName, month := range monthMap {
		if strings.Contains(strings.ToUpper(couponCode), monthName) {
			// Extract year and day
			re := regexp.MustCompile(`(\d{1,2})?` + monthName + `(\d{4})`)
			matches := re.FindStringSubmatch(strings.ToUpper(couponCode))
			if len(matches) >= 3 {
				year, _ := strconv.Atoi(matches[2])
				day := 1
				if matches[1] != "" {
					day, _ = strconv.Atoi(matches[1])
				}
				if year > 0 && year >= time.Now().Year() && day > 0 && day <= 31 {
					return time.Date(year, month, day, 23, 59, 59, 0, time.UTC)
				}
			}
		}
	}
	
	// Look for just year (like "2025") - assume end of year
	re := regexp.MustCompile(`20\d{2}`)
	if matches := re.FindStringSubmatch(couponCode); len(matches) > 0 {
		if year, err := strconv.Atoi(matches[0]); err == nil && year >= time.Now().Year() {
			return time.Date(year, time.December, 31, 23, 59, 59, 0, time.UTC)
		}
	}
	
	return time.Time{} // Zero time if no date found
}

func (s *Scraper) calculateQualityScore(rating float64, studentCount int, title, description string) float64 {
	var score float64
	
	// Base score from rating (0-40 points)
	if rating > 0 {
		score += rating * 8 // 5.0 rating = 40 points
	}
	
	// Student count bonus (0-30 points)
	switch {
	case studentCount >= 1000:
		score += 30
	case studentCount >= 500:
		score += 25
	case studentCount >= 100:
		score += 20
	case studentCount >= 50:
		score += 15
	case studentCount >= 10:
		score += 10
	case studentCount > 0:
		score += 5
	}
	
	// Title quality indicators (0-15 points)
	titleLower := strings.ToLower(title)
	
	// Positive indicators
	positiveWords := []string{
		"complete", "comprehensive", "masterclass", "bootcamp", "advanced", 
		"professional", "certification", "diploma", "course", "guide",
		"tutorial", "training", "learn", "master", "expert",
	}
	for _, word := range positiveWords {
		if strings.Contains(titleLower, word) {
			score += 2
		}
	}
	
	// Negative indicators (reduce score)
	negativeWords := []string{
		"quick", "crash", "basics only", "intro", "beginner only",
		"summary", "overview", "brief",
	}
	for _, word := range negativeWords {
		if strings.Contains(titleLower, word) {
			score -= 3
		}
	}
	
	// Description quality (0-10 points)
	if len(description) > 100 {
		score += 5 // Detailed description
	}
	if len(description) > 200 {
		score += 3 // Very detailed description
	}
	
	// Year/recency bonus (0-5 points)
	currentYear := time.Now().Year()
	for year := currentYear; year >= currentYear-2; year-- {
		if strings.Contains(title, strconv.Itoa(year)) {
			score += float64(3 - (currentYear - year)) // 2025=3pts, 2024=2pts, 2023=1pt
			break
		}
	}
	
	// Cap the score at 100
	if score > 100 {
		score = 100
	}
	
	// Ensure minimum score of 0
	if score < 0 {
		score = 0
	}
	
	return score
}