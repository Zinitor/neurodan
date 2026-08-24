package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aeremin/neurodan/internal/telegram"
)

func main() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN environment variable is required")
	}

	generatorURL := os.Getenv("GENERATOR_URL")
	if generatorURL == "" {
		generatorURL = "http://localhost:8000"
	}

	cfg := telegram.Config{
		ReplyBase:       envFloat("REPLY_BASE", 0.10),
		ReplyGrowth:     envFloat("REPLY_GROWTH", 0.10),
		ReplyMax:        envFloat("REPLY_MAX", 0.60),
		ChanceBasedReplyProb: envFloat("CHANCE_BASED_REPLY_PROB", 0.20),
		ChainMax:        envInt("CHAIN_MAX", 3),
		Length:          getEnv("LENGTH", "medium"),
		WindowSize:      envInt("WINDOW_SIZE", 15),
		MaxContextWords: envInt("MAX_CONTEXT_WORDS", 300),
	}

	settings, err := telegram.NewSettingsStore(getEnv("SETTINGS_FILE", "./data/chat_settings.json"))
	if err != nil {
		log.Fatalf("Failed to load settings store: %v", err)
	}

	fmt.Printf("Bot starting, connecting to generator at %s\n", generatorURL)

	client := telegram.NewClient(token, generatorURL, cfg, settings)
	client.Start()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil || os.Getenv(key) == "" {
		return fallback
	}
	return v
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || os.Getenv(key) == "" {
		return fallback
	}
	return v
}
