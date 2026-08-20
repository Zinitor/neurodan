package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Client struct {
	bot          *tgbotapi.BotAPI
	generatorURL string
}

type trainRequest struct {
	Text   string `json:"text"`
	ChatID int64  `json:"chat_id"`
}

type generateRequest struct {
	Length int    `json:"length"`
	Seed   string `json:"seed,omitempty"`
	ChatID int64  `json:"chat_id"`
}

type generateResponse struct {
	Text string `json:"text"`
}

func NewClient(token, generatorURL string) *Client {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		panic(fmt.Sprintf("failed to create bot: %v", err))
	}

	return &Client{
		bot:          bot,
		generatorURL: generatorURL,
	}
}

func (c *Client) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := c.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			c.handleCommand(update.Message)
		} else {
			c.handleMessage(update.Message)
		}
	}
}

func (c *Client) handleMessage(msg *tgbotapi.Message) {
	if msg.Text == "" {
		return
	}

	go c.train(msg.Chat.ID, msg.Text)
}

func (c *Client) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "generate":
		seed := strings.TrimSpace(msg.CommandArguments())
		c.generate(msg.Chat.ID, seed)
	default:
	}
}

func (c *Client) train(chatID int64, text string) error {
	req := trainRequest{Text: text, ChatID: chatID}
	return c.post("/train", req, nil)
}

func (c *Client) generate(chatID int64, seed string) {
	req := generateRequest{Length: 10, Seed: seed, ChatID: chatID}
	var resp generateResponse

	err := c.post("/generate", req, &resp)
	if err != nil {
		reply := tgbotapi.NewMessage(chatID, "Error generating text")
		c.bot.Send(reply)
		return
	}

	reply := tgbotapi.NewMessage(chatID, resp.Text)
	c.bot.Send(reply)
}

func (c *Client) post(path string, reqBody interface{}, respBody interface{}) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := http.Post(c.generatorURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("generator returned %d: %s", resp.StatusCode, string(body))
	}

	if respBody != nil {
		return json.NewDecoder(resp.Body).Decode(respBody)
	}
	return nil
}