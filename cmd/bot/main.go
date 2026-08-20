package main

import (
	"fmt"
	"log"
	"os"

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

	fmt.Printf("Bot starting, connecting to generator at %s\n", generatorURL)

	client := telegram.NewClient(token, generatorURL)
	client.Start()
}