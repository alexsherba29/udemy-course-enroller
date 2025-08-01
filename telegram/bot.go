package telegram

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"udemy-course-notifier/database"
	"udemy-course-notifier/filters"
	"udemy-course-notifier/security"
)

type Bot struct {
	api           *tgbotapi.BotAPI
	db            *database.DB
	channelID     string
	filterEngine  *filters.FilterEngine
	awaitingInput map[int64]string // Track users awaiting filter input
}

func New(token, channelID string, db *database.DB) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	api.Debug = false

	return &Bot{
		api:           api,
		db:            db,
		channelID:     channelID,
		filterEngine:  filters.New(db),
		awaitingInput: make(map[int64]string),
	}, nil
}

func (b *Bot) Start() error {
	log.Printf("Authorized on account %s", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			b.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			b.handleCallbackQuery(update.CallbackQuery)
		}
	}

	return nil
}

func (b *Bot) handleMessage(message *tgbotapi.Message) {
	userID := message.From.ID
	
	// Check if user is in filter input mode
	if inputType, exists := b.awaitingInput[userID]; exists {
		b.handleFilterInput(message, inputType)
		return
	}

	if !message.IsCommand() {
		return
	}

	command := message.Command()
	args := message.CommandArguments()

	switch command {
	case "start":
		b.handleStartCommand(message)
	case "help":
		b.handleHelpCommand(message)
	case "filter":
		b.handleFilterCommand(message, args)
	case "wishlist":
		b.handleWishlistCommand(message)
	case "stats":
		b.handleStatsCommand(message)
	default:
		b.sendMessage(message.Chat.ID, "Unknown command. Use /help to see available commands.")
	}
}

func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 2 {
		return
	}

	action := parts[0]
	courseIDStr := parts[1]
	courseID, err := strconv.Atoi(courseIDStr)
	if err != nil {
		return
	}

	userID := callback.From.ID

	switch action {
	case "ignore":
		if err := b.db.IgnoreCourse(userID, courseID); err != nil {
			log.Printf("Failed to ignore course: %v", err)
			return
		}
		
		// Edit message to show it's been ignored
		edit := tgbotapi.NewEditMessageText(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			callback.Message.Text+"\n\n✅ *Marked as not interested*",
		)
		edit.ParseMode = "Markdown"
		b.api.Send(edit)

	case "wishlist":
		if err := b.db.AddToWishlist(userID, courseID); err != nil {
			log.Printf("Failed to add to wishlist: %v", err)
			return
		}
		
		// Edit message to show it's been added to wishlist
		edit := tgbotapi.NewEditMessageText(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			callback.Message.Text+"\n\n⭐ *Added to wishlist*",
		)
		edit.ParseMode = "Markdown"
		b.api.Send(edit)

	case "remove_wishlist":
		if err := b.db.RemoveFromWishlist(userID, courseID); err != nil {
			log.Printf("Failed to remove from wishlist: %v", err)
			return
		}
		
		// Edit message to show it's been removed from wishlist
		edit := tgbotapi.NewEditMessageText(
			callback.Message.Chat.ID,
			callback.Message.MessageID,
			callback.Message.Text+"\n\n🗑️ *Removed from wishlist*",
		)
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
	}

	// Answer callback query to remove loading state
	answer := tgbotapi.NewCallback(callback.ID, "")
	b.api.Request(answer)
}

func (b *Bot) handleStartCommand(message *tgbotapi.Message) {
	text := `Welcome to the Free Udemy Course Notifier! 🎓

I'll help you discover free Udemy courses based on your interests.

Available commands:
/filter - Set your course preferences
/wishlist - View your saved courses
/stats - View your activity stats
/help - Show this help message

You can also use the buttons on course messages to:
• Add courses to your wishlist ⭐
• Mark courses as not interested ❌`

	b.sendMessage(message.Chat.ID, text)
}

func (b *Bot) handleHelpCommand(message *tgbotapi.Message) {
	text := `📚 *Free Udemy Course Notifier Help*

*Commands:*
/start - Welcome message and setup
/filter - Configure your course preferences
/wishlist - View courses you've saved
/stats - See your activity statistics
/help - Show this help message

*How it works:*
1. I monitor public sources for free Udemy courses
2. I filter courses based on your preferences
3. You get notified about relevant courses
4. Use buttons to save or ignore courses

*Tips:*
• Set up your preferences with /filter for better recommendations
• Use the wishlist to save interesting courses for later
• Mark courses as "not interested" to improve future suggestions`

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) handleFilterCommand(message *tgbotapi.Message, args string) {
	if args != "" {
		// Process filter arguments directly
		b.processFilterInput(message.From.ID, message.Chat.ID, args)
		return
	}

	// Request filter input from user
	text := `🎯 *Course Filter Settings*

Please send your preferences in this format:
` + "`Categories | MinRating | Keywords | ExcludedKeywords`" + `

*Example:*
` + "`Development, Business | 4.0 | programming, web | crypto, trading`" + `

*Categories:* Development, Business, Design, Marketing, IT & Software, etc.
*MinRating:* 0.0 to 5.0
*Keywords:* Topics you want (comma-separated)
*ExcludedKeywords:* Topics to avoid (comma-separated)

Send your preferences now:`

	b.awaitingInput[message.From.ID] = "filter"
	
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) handleFilterInput(message *tgbotapi.Message, inputType string) {
	userID := message.From.ID
	delete(b.awaitingInput, userID) // Remove from waiting list

	if inputType == "filter" {
		b.processFilterInput(userID, message.Chat.ID, message.Text)
	}
}

func (b *Bot) processFilterInput(userID int64, chatID int64, input string) {
	// Validate and sanitize input
	if err := security.ValidateFilterString(input); err != nil {
		b.sendMessage(chatID, "❌ Invalid filter format. Please check your input and try again.")
		return
	}

	sanitizedInput := security.SanitizeString(input)
	userFilter := filters.ParseFilterString(userID, sanitizedInput)
	
	if err := b.filterEngine.SaveUserFilter(userFilter); err != nil {
		b.sendMessage(chatID, "❌ Failed to save your preferences. Please try again.")
		log.Printf("Failed to save user filter: %v", err)
		return
	}

	text := fmt.Sprintf(`✅ *Filter preferences saved!*

📂 Categories: %v
⭐ Min Rating: %.1f
🔍 Keywords: %v
❌ Excluded: %v

You'll now receive notifications for courses matching these criteria.`,
		userFilter.Categories,
		userFilter.MinRating,
		userFilter.Keywords,
		userFilter.ExcludedKeywords,
	)

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) handleWishlistCommand(message *tgbotapi.Message) {
	userID := message.From.ID
	
	// Get user's wishlist
	wishlist, err := b.getUserWishlist(userID)
	if err != nil {
		b.sendMessage(message.Chat.ID, "❌ Failed to retrieve your wishlist.")
		log.Printf("Failed to get wishlist: %v", err)
		return
	}

	if len(wishlist) == 0 {
		text := `⭐ *Your Wishlist*

Your wishlist is empty. 
You can add courses to your wishlist by clicking the ⭐ button on course notifications.`

		msg := tgbotapi.NewMessage(message.Chat.ID, text)
		msg.ParseMode = "Markdown"
		b.api.Send(msg)
		return
	}

	// Send courses with remove buttons (limit to 5 at a time due to message length)
	coursesToShow := len(wishlist)
	if coursesToShow > 5 {
		coursesToShow = 5
	}
	
	for i := 0; i < coursesToShow; i++ {
		course := wishlist[i]
		courseText := fmt.Sprintf("🎓 *%s*\n📂 %s | ⭐ %.1f\n🔗 %s",
			course.Title, course.Category, course.Rating, course.URL)
		
		// Create remove button for each course
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑️ Remove from Wishlist", fmt.Sprintf("remove_wishlist:%d", course.ID)),
				tgbotapi.NewInlineKeyboardButtonURL("🔗 View Course", course.URL),
			),
		)
		
		msg := tgbotapi.NewMessage(message.Chat.ID, courseText)
		msg.ParseMode = "Markdown"
		msg.ReplyMarkup = keyboard
		msg.DisableWebPagePreview = true
		b.api.Send(msg)
	}
	
	// If there are more courses, show summary
	if len(wishlist) > 5 {
		summaryText := fmt.Sprintf("\n... and %d more courses in your wishlist.\nUse /wishlist again to see more.", len(wishlist)-5)
		summaryMsg := tgbotapi.NewMessage(message.Chat.ID, summaryText)
		b.api.Send(summaryMsg)
	}
}

func (b *Bot) handleStatsCommand(message *tgbotapi.Message) {
	userID := message.From.ID
	
	// Get user statistics
	wishlistCount, err := b.getWishlistCount(userID)
	if err != nil {
		wishlistCount = 0
	}
	
	ignoredCount, err := b.getIgnoredCount(userID)
	if err != nil {
		ignoredCount = 0
	}

	text := fmt.Sprintf(`📊 *Your Activity Stats*

⭐ Courses in wishlist: %d
❌ Courses ignored: %d
🎯 Filter preferences: %s

Use /wishlist to view saved courses
Use /filter to update preferences`,
		wishlistCount,
		ignoredCount,
		b.getFilterStatus(userID),
	)

	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) PostCourse(course *database.Course) error {
	text := b.formatCourseMessage(course)
	
	// Create inline keyboard with action buttons
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⭐ Save", fmt.Sprintf("wishlist:%d", course.ID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Not Interested", fmt.Sprintf("ignore:%d", course.ID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("🔗 View Course", course.URL),
		),
	)

	// Send to channel
	channelID, err := strconv.ParseInt(b.channelID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid channel ID: %w", err)
	}

	msg := tgbotapi.NewMessage(channelID, text)
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = keyboard
	msg.DisableWebPagePreview = true

	_, err = b.api.Send(msg)
	return err
}

func (b *Bot) formatCourseMessage(course *database.Course) string {
	expiresIn := time.Until(course.ExpiresAt)
	expiry := "Unknown"
	urgencyIcon := "🕒"
	
	if expiresIn > 0 {
		hours := expiresIn.Hours()
		if hours < 6 {
			expiry = fmt.Sprintf("%.0f hours", hours)
			urgencyIcon = "🚨" // Very urgent
		} else if hours < 24 {
			expiry = fmt.Sprintf("%.0f hours", hours)
			urgencyIcon = "⚡" // Urgent
		} else {
			days := hours / 24
			expiry = fmt.Sprintf("%.0f days", days)
			if days <= 3 {
				urgencyIcon = "⏰" // Somewhat urgent
			} else {
				urgencyIcon = "🕒" // Normal
			}
		}
	}

	// Quality score indicator
	qualityIcon := "🔴" // Low quality
	if course.QualityScore >= 80 {
		qualityIcon = "🟢" // High quality
	} else if course.QualityScore >= 60 {
		qualityIcon = "🟡" // Medium quality
	} else if course.QualityScore >= 40 {
		qualityIcon = "🟠" // Fair quality
	}

	rating := ""
	if course.Rating > 0 {
		rating = fmt.Sprintf("⭐ %.1f", course.Rating)
	}

	students := ""
	if course.StudentCount > 0 {
		if course.StudentCount >= 1000 {
			students = fmt.Sprintf("👥 %dk students", course.StudentCount/1000)
		} else {
			students = fmt.Sprintf("👥 %d students", course.StudentCount)
		}
	}

	text := fmt.Sprintf(`🎓 *%s*

📂 Category: %s
💰 Price: %s %s
%s Expires in: %s
%s Quality Score: %.0f/100
%s %s

%s`,
		course.Title,
		course.Category,
		course.Price,
		course.Discount,
		urgencyIcon,
		expiry,
		qualityIcon,
		course.QualityScore,
		rating,
		students,
		course.Description,
	)

	return text
}

func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}

func (b *Bot) getUserWishlist(userID int64) ([]database.Course, error) {
	query := `SELECT c.id, c.url, c.title, c.description, c.category, c.rating, c.price, c.discount, c.expires_at, c.posted_at, c.quality_score, c.student_count 
			  FROM courses c
			  INNER JOIN wishlist w ON c.id = w.course_id
			  WHERE w.user_id = ?
			  ORDER BY w.added_at DESC`
	
	rows, err := b.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query wishlist: %w", err)
	}
	defer rows.Close()
	
	var courses []database.Course
	for rows.Next() {
		var course database.Course
		err := rows.Scan(&course.ID, &course.URL, &course.Title, &course.Description,
			&course.Category, &course.Rating, &course.Price, &course.Discount,
			&course.ExpiresAt, &course.PostedAt, &course.QualityScore, &course.StudentCount)
		if err != nil {
			log.Printf("Failed to scan course: %v", err)
			continue
		}
		courses = append(courses, course)
	}
	
	return courses, nil
}


func (b *Bot) getWishlistCount(userID int64) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM wishlist WHERE user_id = ?`
	err := b.db.QueryRow(query, userID).Scan(&count)
	return count, err
}

func (b *Bot) getIgnoredCount(userID int64) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM ignored_courses WHERE user_id = ?`
	err := b.db.QueryRow(query, userID).Scan(&count)
	return count, err
}

func (b *Bot) getFilterStatus(userID int64) string {
	filter, err := b.filterEngine.GetUserFilter(userID)
	if err != nil {
		return "Not set"
	}
	
	status := ""
	if len(filter.Categories) > 0 {
		status += fmt.Sprintf("Categories: %d, ", len(filter.Categories))
	}
	if filter.MinRating > 0 {
		status += fmt.Sprintf("Min Rating: %.1f, ", filter.MinRating)
	}
	if len(filter.Keywords) > 0 {
		status += fmt.Sprintf("Keywords: %d", len(filter.Keywords))
	}
	
	if status == "" {
		return "Not set"
	}
	
	return strings.TrimSuffix(status, ", ")
}