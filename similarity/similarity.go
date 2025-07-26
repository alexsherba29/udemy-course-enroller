package similarity

import (
	"math"
	"regexp"
	"strings"
	"udemy-course-notifier/database"
)

// SimilarityEngine handles course deduplication and similarity detection
type SimilarityEngine struct {
	similarityThreshold float64
}

// New creates a new similarity engine
func New(threshold float64) *SimilarityEngine {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.85 // Default 85% similarity threshold
	}
	return &SimilarityEngine{
		similarityThreshold: threshold,
	}
}

// IsSimilar checks if two courses are similar enough to be considered duplicates
func (se *SimilarityEngine) IsSimilar(course1, course2 *database.Course) bool {
	similarity := se.CalculateSimilarity(course1, course2)
	return similarity >= se.similarityThreshold
}

// CalculateSimilarity returns a similarity score between 0 and 1
func (se *SimilarityEngine) CalculateSimilarity(course1, course2 *database.Course) float64 {
	// Title similarity (weighted 60%)
	titleSim := se.calculateTextSimilarity(course1.Title, course2.Title) * 0.6
	
	// Description similarity (weighted 20%)
	descSim := se.calculateTextSimilarity(course1.Description, course2.Description) * 0.2
	
	// Category similarity (weighted 20%)
	categorySim := 0.0
	if strings.ToLower(course1.Category) == strings.ToLower(course2.Category) {
		categorySim = 0.2
	}
	
	totalSimilarity := titleSim + descSim + categorySim
	
	// Bonus for similar ratings (within 0.5 points)
	if math.Abs(course1.Rating-course2.Rating) <= 0.5 {
		totalSimilarity += 0.05
	}
	
	// Bonus for similar student counts (within 20%)
	if course1.StudentCount > 0 && course2.StudentCount > 0 {
		ratio := float64(min(course1.StudentCount, course2.StudentCount)) / 
				 float64(max(course1.StudentCount, course2.StudentCount))
		if ratio >= 0.8 {
			totalSimilarity += 0.05
		}
	}
	
	return math.Min(totalSimilarity, 1.0)
}

// FindBestCourse returns the better course from a similar pair
func (se *SimilarityEngine) FindBestCourse(course1, course2 *database.Course) *database.Course {
	// Compare by quality score first
	if course1.QualityScore != course2.QualityScore {
		if course1.QualityScore > course2.QualityScore {
			return course1
		}
		return course2
	}
	
	// If quality scores are equal, compare by rating
	if course1.Rating != course2.Rating {
		if course1.Rating > course2.Rating {
			return course1
		}
		return course2
	}
	
	// If ratings are equal, compare by student count
	if course1.StudentCount != course2.StudentCount {
		if course1.StudentCount > course2.StudentCount {
			return course1
		}
		return course2
	}
	
	// If all else is equal, return the more recent one
	if course1.PostedAt.After(course2.PostedAt) {
		return course1
	}
	return course2
}

// DeduplicateCourses removes similar courses from a slice, keeping only the best version
func (se *SimilarityEngine) DeduplicateCourses(courses []database.Course) []database.Course {
	if len(courses) <= 1 {
		return courses
	}
	
	var deduplicated []database.Course
	processed := make(map[int]bool)
	
	for i, course1 := range courses {
		if processed[i] {
			continue
		}
		
		bestCourse := course1
		processed[i] = true
		
		// Check against all remaining courses
		for j := i + 1; j < len(courses); j++ {
			if processed[j] {
				continue
			}
			
			course2 := courses[j]
			if se.IsSimilar(&bestCourse, &course2) {
				// Found a similar course, keep the better one
				betterCourse := se.FindBestCourse(&bestCourse, &course2)
				if betterCourse.ID == course2.ID {
					bestCourse = course2
				}
				processed[j] = true
			}
		}
		
		deduplicated = append(deduplicated, bestCourse)
	}
	
	return deduplicated
}

// calculateTextSimilarity uses Jaccard similarity on normalized text
func (se *SimilarityEngine) calculateTextSimilarity(text1, text2 string) float64 {
	if text1 == text2 {
		return 1.0
	}
	
	if text1 == "" || text2 == "" {
		return 0.0
	}
	
	// Normalize texts
	norm1 := se.normalizeText(text1)
	norm2 := se.normalizeText(text2)
	
	if norm1 == norm2 {
		return 1.0
	}
	
	// Convert to word sets
	words1 := se.getWordSet(norm1)
	words2 := se.getWordSet(norm2)
	
	// Calculate Jaccard similarity
	intersection := 0
	for word := range words1 {
		if words2[word] {
			intersection++
		}
	}
	
	union := len(words1) + len(words2) - intersection
	if union == 0 {
		return 0.0
	}
	
	return float64(intersection) / float64(union)
}

// normalizeText cleans and normalizes text for comparison
func (se *SimilarityEngine) normalizeText(text string) string {
	// Convert to lowercase
	text = strings.ToLower(text)
	
	// Remove common course prefixes/suffixes
	commonPrefixes := []string{
		"complete", "comprehensive", "ultimate", "full", "total", "entire",
		"master", "mastering", "learn", "learning", "course", "tutorial",
		"guide", "introduction", "intro", "advanced", "beginner", "basic",
		"professional", "pro", "expert", "bootcamp", "training",
	}
	
	for _, prefix := range commonPrefixes {
		// Remove as prefix
		if strings.HasPrefix(text, prefix+" ") {
			text = strings.TrimPrefix(text, prefix+" ")
		}
		// Remove as suffix
		if strings.HasSuffix(text, " "+prefix) {
			text = strings.TrimSuffix(text, " "+prefix)
		}
		// Remove standalone
		text = regexp.MustCompile(`\b`+regexp.QuoteMeta(prefix)+`\b`).ReplaceAllString(text, "")
	}
	
	// Remove years (2024, 2025, etc.)
	yearRegex := regexp.MustCompile(`\b20\d{2}\b`)
	text = yearRegex.ReplaceAllString(text, "")
	
	// Remove special characters and normalize whitespace
	text = regexp.MustCompile(`[^\p{L}\p{N}\s]`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	
	return strings.TrimSpace(text)
}

// getWordSet converts text to a set of words
func (se *SimilarityEngine) getWordSet(text string) map[string]bool {
	words := strings.Fields(text)
	wordSet := make(map[string]bool)
	
	for _, word := range words {
		// Skip very short words
		if len(word) >= 3 {
			wordSet[word] = true
		}
	}
	
	return wordSet
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}