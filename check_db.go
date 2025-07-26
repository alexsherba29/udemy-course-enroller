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

	courses, err := db.GetRecentCourses(100)
	if err != nil {
		log.Fatalf("Failed to get courses: %v", err)
	}

	fmt.Printf("Found %d courses in database:\n", len(courses))
	for i, course := range courses {
		fmt.Printf("%d. %s\n   URL: %s\n   Category: %s | Rating: %.1f\n\n", 
			i+1, course.Title, course.URL, course.Category, course.Rating)
	}
}