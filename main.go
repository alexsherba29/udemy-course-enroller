package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"udemy-course-notifier/config"
	"udemy-course-notifier/database"
	"udemy-course-notifier/logger"
	"udemy-course-notifier/scraper"
	"udemy-course-notifier/similarity"
	"udemy-course-notifier/telegram"
)

func main() {
	log.Println("Starting Udemy Course Notifier Bot...")

	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.New(cfg.Logging.File, cfg.Logging.Level)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Close()

	appLogger.Info("Starting Udemy Course Notifier Bot...")

	// Initialize database
	db, err := database.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize Telegram bot
	bot, err := telegram.New(cfg.Telegram.Token, cfg.Telegram.ChannelID, db)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	// Initialize scraper
	courseScraper := scraper.New(cfg.Scraping.UserAgent, cfg.Scraping.RateLimitDelaySeconds)

	// Start course monitoring in a separate goroutine
	go startCourseMonitoring(cfg, courseScraper, db, bot)

	// Start bot in a separate goroutine
	go func() {
		if err := bot.Start(); err != nil {
			log.Printf("Bot error: %v", err)
		}
	}()

	log.Println("Bot started successfully!")

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down gracefully...")
}

func startCourseMonitoring(cfg *config.Config, scraper *scraper.Scraper, db *database.DB, bot *telegram.Bot) {
	ticker := time.NewTicker(time.Duration(cfg.Scraping.IntervalMinutes) * time.Minute)
	defer ticker.Stop()

	// Run initial scan
	scanForCourses(cfg, scraper, db, bot)

	for range ticker.C {
		scanForCourses(cfg, scraper, db, bot)
	}
}

func scanForCourses(cfg *config.Config, scraper *scraper.Scraper, db *database.DB, bot *telegram.Bot) {
	log.Println("Scanning for new courses...")

	// Initialize similarity engine
	similarityEngine := similarity.New(0.85) // 85% similarity threshold
	var allNewCourses []database.Course

	for _, sourceURL := range cfg.Scraping.SourceURLs {
		courses, err := scraper.ScrapeCoursesFromURL(sourceURL)
		if err != nil {
			log.Printf("Failed to scrape %s: %v", sourceURL, err)
			continue
		}

		// Filter out existing courses
		var newCourses []database.Course
		for _, course := range courses {
			exists, err := db.CourseExists(course.URL)
			if err != nil {
				log.Printf("Failed to check if course exists: %v", err)
				continue
			}

			if !exists {
				newCourses = append(newCourses, course)
			}
		}

		allNewCourses = append(allNewCourses, newCourses...)
	}

	// Deduplicate courses across all sources
	log.Printf("Found %d new courses before deduplication", len(allNewCourses))
	deduplicatedCourses := similarityEngine.DeduplicateCourses(allNewCourses)
	log.Printf("After deduplication: %d unique courses", len(deduplicatedCourses))

	// Process deduplicated courses
	for _, course := range deduplicatedCourses {
		// Add course to database
		if err := db.AddCourse(&course); err != nil {
			log.Printf("Failed to add course to database: %v", err)
			continue
		}

		// Post to Telegram channel
		if err := bot.PostCourse(&course); err != nil {
			log.Printf("Failed to post course to Telegram: %v", err)
		} else {
			log.Printf("Posted new course: %s (Quality: %.1f)", course.Title, course.QualityScore)
		}

		// Rate limiting between posts
		time.Sleep(2 * time.Second)
	}

	log.Println("Course scan completed")
}