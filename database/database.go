package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

type Course struct {
	ID           int       `json:"id"`
	URL          string    `json:"url"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Category     string    `json:"category"`
	Rating       float64   `json:"rating"`
	Price        string    `json:"price"`
	Discount     string    `json:"discount"`
	ExpiresAt    time.Time `json:"expires_at"`
	PostedAt     time.Time `json:"posted_at"`
	QualityScore float64   `json:"quality_score"`
	StudentCount int       `json:"student_count"`
}

type UserPreference struct {
	UserID           int64    `json:"user_id"`
	Categories       []string `json:"categories"`
	Keywords         []string `json:"keywords"`
	ExcludedKeywords []string `json:"excluded_keywords"`
	MinRating        float64  `json:"min_rating"`
	Language         string   `json:"language"`
}

type WishlistItem struct {
	ID       int       `json:"id"`
	UserID   int64     `json:"user_id"`
	CourseID int       `json:"course_id"`
	AddedAt  time.Time `json:"added_at"`
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

func (db *DB) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS courses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT UNIQUE NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			category TEXT,
			rating REAL,
			price TEXT,
			discount TEXT,
			expires_at DATETIME,
			posted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			quality_score REAL DEFAULT 0,
			student_count INTEGER DEFAULT 0
		)`,
		
		`CREATE TABLE IF NOT EXISTS user_preferences (
			user_id INTEGER PRIMARY KEY,
			categories TEXT,
			keywords TEXT,
			excluded_keywords TEXT,
			min_rating REAL DEFAULT 0.0,
			language TEXT DEFAULT 'en'
		)`,
		
		`CREATE TABLE IF NOT EXISTS wishlist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			course_id INTEGER NOT NULL,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (course_id) REFERENCES courses(id),
			UNIQUE(user_id, course_id)
		)`,
		
		`CREATE TABLE IF NOT EXISTS ignored_courses (
			user_id INTEGER NOT NULL,
			course_id INTEGER NOT NULL,
			ignored_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (course_id) REFERENCES courses(id),
			PRIMARY KEY (user_id, course_id)
		)`,
	}

	for _, query := range queries {
		if _, err := db.conn.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	return nil
}

func (db *DB) AddCourse(course *Course) error {
	query := `INSERT INTO courses (url, title, description, category, rating, price, discount, expires_at, quality_score, student_count) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	result, err := db.conn.Exec(query, course.URL, course.Title, course.Description, 
		course.Category, course.Rating, course.Price, course.Discount, course.ExpiresAt,
		course.QualityScore, course.StudentCount)
	if err != nil {
		return fmt.Errorf("failed to insert course: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}
	
	course.ID = int(id)
	return nil
}

func (db *DB) CourseExists(url string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM courses WHERE url = ?)`
	err := db.conn.QueryRow(query, url).Scan(&exists)
	return exists, err
}

func (db *DB) CleanupOldCourses(daysOld int) error {
	query := `DELETE FROM courses WHERE posted_at < datetime('now', '-' || ? || ' days')`
	_, err := db.conn.Exec(query, daysOld)
	if err != nil {
		return fmt.Errorf("failed to cleanup old courses: %w", err)
	}
	return nil
}

func (db *DB) GetRecentCourses(limit int) ([]Course, error) {
	query := `SELECT id, url, title, description, category, rating, price, discount, expires_at, posted_at, quality_score, student_count 
			  FROM courses ORDER BY posted_at DESC LIMIT ?`
	
	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query courses: %w", err)
	}
	defer rows.Close()

	var courses []Course
	for rows.Next() {
		var course Course
		err := rows.Scan(&course.ID, &course.URL, &course.Title, &course.Description,
			&course.Category, &course.Rating, &course.Price, &course.Discount,
			&course.ExpiresAt, &course.PostedAt, &course.QualityScore, &course.StudentCount)
		if err != nil {
			return nil, fmt.Errorf("failed to scan course: %w", err)
		}
		courses = append(courses, course)
	}

	return courses, nil
}

func (db *DB) AddToWishlist(userID int64, courseID int) error {
	query := `INSERT INTO wishlist (user_id, course_id) VALUES (?, ?)`
	_, err := db.conn.Exec(query, userID, courseID)
	if err != nil {
		return fmt.Errorf("failed to add to wishlist: %w", err)
	}
	return nil
}

func (db *DB) RemoveFromWishlist(userID int64, courseID int) error {
	query := `DELETE FROM wishlist WHERE user_id = ? AND course_id = ?`
	_, err := db.conn.Exec(query, userID, courseID)
	if err != nil {
		return fmt.Errorf("failed to remove from wishlist: %w", err)
	}
	return nil
}

func (db *DB) IgnoreCourse(userID int64, courseID int) error {
	query := `INSERT INTO ignored_courses (user_id, course_id) VALUES (?, ?)`
	_, err := db.conn.Exec(query, userID, courseID)
	if err != nil {
		return fmt.Errorf("failed to ignore course: %w", err)
	}
	return nil
}

func (db *DB) IsIgnored(userID int64, courseID int) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM ignored_courses WHERE user_id = ? AND course_id = ?)`
	err := db.conn.QueryRow(query, userID, courseID).Scan(&exists)
	return exists, err
}

func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

func (db *DB) Close() error {
	return db.conn.Close()
}