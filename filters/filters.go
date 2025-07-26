package filters

import (
	"encoding/json"
	"strings"

	"udemy-course-notifier/database"
)

type UserFilter struct {
	UserID           int64    `json:"user_id"`
	Categories       []string `json:"categories"`
	Keywords         []string `json:"keywords"`
	ExcludedKeywords []string `json:"excluded_keywords"`
	MinRating        float64  `json:"min_rating"`
	Language         string   `json:"language"`
}

type FilterEngine struct {
	db *database.DB
}

func New(db *database.DB) *FilterEngine {
	return &FilterEngine{db: db}
}

func (f *FilterEngine) ShouldNotifyCourse(course *database.Course, userID int64) (bool, error) {
	// Check if user has ignored this course
	ignored, err := f.db.IsIgnored(userID, course.ID)
	if err != nil {
		return false, err
	}
	if ignored {
		return false, nil
	}

	// Get user preferences
	userFilter, err := f.getUserFilter(userID)
	if err != nil {
		return true, nil // Default to showing course if no preferences set
	}

	// Apply filters
	if !f.matchesCategories(course, userFilter.Categories) {
		return false, nil
	}

	if !f.matchesKeywords(course, userFilter.Keywords) {
		return false, nil
	}

	if f.containsExcludedKeywords(course, userFilter.ExcludedKeywords) {
		return false, nil
	}

	if course.Rating < userFilter.MinRating {
		return false, nil
	}

	return true, nil
}

func (f *FilterEngine) SaveUserFilter(userFilter *UserFilter) error {
	categoriesJSON, _ := json.Marshal(userFilter.Categories)
	keywordsJSON, _ := json.Marshal(userFilter.Keywords)
	excludedJSON, _ := json.Marshal(userFilter.ExcludedKeywords)

	query := `INSERT OR REPLACE INTO user_preferences 
			  (user_id, categories, keywords, excluded_keywords, min_rating, language) 
			  VALUES (?, ?, ?, ?, ?, ?)`

	_, err := f.db.Exec(query, userFilter.UserID, string(categoriesJSON), 
		string(keywordsJSON), string(excludedJSON), userFilter.MinRating, userFilter.Language)
	
	return err
}

func (f *FilterEngine) GetUserFilter(userID int64) (*UserFilter, error) {
	return f.getUserFilter(userID)
}

func (f *FilterEngine) getUserFilter(userID int64) (*UserFilter, error) {
	query := `SELECT categories, keywords, excluded_keywords, min_rating, language 
			  FROM user_preferences WHERE user_id = ?`

	var categoriesJSON, keywordsJSON, excludedJSON string
	var minRating float64
	var language string

	err := f.db.QueryRow(query, userID).Scan(&categoriesJSON, &keywordsJSON, 
		&excludedJSON, &minRating, &language)
	if err != nil {
		return nil, err
	}

	userFilter := &UserFilter{
		UserID:    userID,
		MinRating: minRating,
		Language:  language,
	}

	json.Unmarshal([]byte(categoriesJSON), &userFilter.Categories)
	json.Unmarshal([]byte(keywordsJSON), &userFilter.Keywords)
	json.Unmarshal([]byte(excludedJSON), &userFilter.ExcludedKeywords)

	return userFilter, nil
}

func (f *FilterEngine) matchesCategories(course *database.Course, categories []string) bool {
	if len(categories) == 0 {
		return true // No category filter
	}

	courseCategory := strings.ToLower(course.Category)
	for _, category := range categories {
		if strings.Contains(courseCategory, strings.ToLower(category)) {
			return true
		}
	}

	return false
}

func (f *FilterEngine) matchesKeywords(course *database.Course, keywords []string) bool {
	if len(keywords) == 0 {
		return true // No keyword filter
	}

	searchText := strings.ToLower(course.Title + " " + course.Description)
	
	for _, keyword := range keywords {
		if strings.Contains(searchText, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

func (f *FilterEngine) containsExcludedKeywords(course *database.Course, excludedKeywords []string) bool {
	if len(excludedKeywords) == 0 {
		return false // No exclusions
	}

	searchText := strings.ToLower(course.Title + " " + course.Description)
	
	for _, keyword := range excludedKeywords {
		if strings.Contains(searchText, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

func ParseFilterString(userID int64, filterStr string) *UserFilter {
	// Parse filter string like: "Development, Business | 4.0 | programming, web | crypto"
	parts := strings.Split(filterStr, "|")
	
	filter := &UserFilter{
		UserID:    userID,
		MinRating: 0.0,
		Language:  "en",
	}

	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		categories := strings.Split(parts[0], ",")
		for i, cat := range categories {
			categories[i] = strings.TrimSpace(cat)
		}
		filter.Categories = categories
	}

	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		if rating := parseFloat(strings.TrimSpace(parts[1])); rating > 0 {
			filter.MinRating = rating
		}
	}

	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		keywords := strings.Split(parts[2], ",")
		for i, kw := range keywords {
			keywords[i] = strings.TrimSpace(kw)
		}
		filter.Keywords = keywords
	}

	if len(parts) > 3 && strings.TrimSpace(parts[3]) != "" {
		excluded := strings.Split(parts[3], ",")
		for i, ex := range excluded {
			excluded[i] = strings.TrimSpace(ex)
		}
		filter.ExcludedKeywords = excluded
	}

	return filter
}

func parseFloat(s string) float64 {
	// Simple float parsing
	if f := 0.0; len(s) > 0 {
		if s[0] >= '0' && s[0] <= '5' {
			f = float64(s[0] - '0')
			if len(s) > 2 && s[1] == '.' && s[2] >= '0' && s[2] <= '9' {
				f += float64(s[2]-'0') / 10.0
			}
		}
		return f
	}
	return 0.0
}