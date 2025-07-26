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

	// Test user ID (your Telegram user ID)
	userID := int64(339096456)
	
	// Get recent courses to add to wishlist
	courses, err := db.GetRecentCourses(5)
	if err != nil {
		log.Fatalf("Failed to get courses: %v", err)
	}
	
	if len(courses) == 0 {
		fmt.Println("No courses found in database")
		return
	}
	
	// Add first course to wishlist
	fmt.Printf("Adding course '%s' to wishlist for user %d\n", courses[0].Title, userID)
	
	err = db.AddToWishlist(userID, courses[0].ID)
	if err != nil {
		log.Printf("Failed to add to wishlist: %v", err)
	} else {
		fmt.Println("✅ Successfully added to wishlist!")
	}
	
	// Check wishlist count
	query := `SELECT COUNT(*) FROM wishlist WHERE user_id = ?`
	var count int
	err = db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		log.Printf("Failed to get wishlist count: %v", err)
	} else {
		fmt.Printf("📊 Wishlist now contains %d courses\n", count)
	}
}