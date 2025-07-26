package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type Update struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
			Title string `json:"title,omitempty"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message,omitempty"`
	ChannelPost *struct {
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
			Title string `json:"title,omitempty"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"channel_post,omitempty"`
}

type Response struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", token)
	
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Failed to get updates: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Println("Recent chats/channels where your bot received messages:")
	fmt.Println("=======================================================")
	
	seen := make(map[int64]bool)
	for _, update := range response.Result {
		var chatID int64
		var chatType, title string
		
		if update.Message != nil {
			chatID = update.Message.Chat.ID
			chatType = update.Message.Chat.Type
			title = update.Message.Chat.Title
		} else if update.ChannelPost != nil {
			chatID = update.ChannelPost.Chat.ID
			chatType = update.ChannelPost.Chat.Type
			title = update.ChannelPost.Chat.Title
		}
		
		if chatID != 0 && !seen[chatID] {
			seen[chatID] = true
			if title == "" {
				title = "Private Chat"
			}
			fmt.Printf("Chat ID: %d\nType: %s\nTitle: %s\n\n", chatID, chatType, title)
		}
	}
	
	if len(seen) == 0 {
		fmt.Println("No recent messages found. Send a message to your bot or add it to a channel first.")
	}
}