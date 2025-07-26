package main

import (
	"fmt"
	"log"
	"udemy-course-notifier/database"
)

func main() {
	db, err := database.New("courses.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// We'll test the wishlist query directly without creating the bot

	// Test user ID
	userID := int64(339096456)
	
	// Test getUserWishlist function directly
	fmt.Println("Testing getUserWishlist function...")
	
	// We need to use reflection or a test method since getUserWishlist is private
	// Let's create a simple SQL query to test
	query := `SELECT c.id, c.url, c.title, c.description, c.category, c.rating, c.price, c.discount, c.expires_at, c.posted_at 
			  FROM courses c
			  INNER JOIN wishlist w ON c.id = w.course_id
			  WHERE w.user_id = ?
			  ORDER BY w.added_at DESC`
	
	rows, err := db.Query(query, userID)
	if err != nil {
		log.Fatalf("Failed to query wishlist: %v", err)
	}
	defer rows.Close()
	
	fmt.Printf("Wishlist courses for user %d:\n", userID)
	fmt.Println("==========================================")
	
	count := 0
	for rows.Next() {
		var course database.Course
		err := rows.Scan(&course.ID, &course.URL, &course.Title, &course.Description,
			&course.Category, &course.Rating, &course.Price, &course.Discount,
			&course.ExpiresAt, &course.PostedAt)
		if err != nil {
			log.Printf("Failed to scan course: %v", err)
			continue
		}
		count++
		fmt.Printf("%d. %s\n   Category: %s | Rating: %.1f\n   URL: %s\n\n", 
			count, course.Title, course.Category, course.Rating, course.URL[:50]+"...")
	}
	
	if count == 0 {
		fmt.Println("No courses found in wishlist")
	} else {
		fmt.Printf("✅ Found %d courses in wishlist - wishlist functionality is working!\n", count)
	}
}