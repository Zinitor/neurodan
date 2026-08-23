package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aeremin/neurodan/internal/markov"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Config holds the tunable behaviour of the bot.
type Config struct {
	// ReplyBase is the probability of replying to a fresh message.
	ReplyBase float64
	// ReplyGrowth is added to the reply probability for every message the bot
	// could have replied to but didn't, so the bot gets more eager over time.
	ReplyGrowth float64
	// ReplyMax caps the stacking reply probability.
	ReplyMax float64
	// ReactionProb is the chance that a triggered reply is a bare reaction
	// (emoji / kaomoji) instead of generated text.
	ReactionProb float64
	// ChainMax is the maximum number of consecutive messages the bot sends in
	// one reply event (always at least 1).
	ChainMax int
	// Length is the length preset sent to the generator: short, medium, long.
	Length string
	// WindowSize is how many recent messages are kept as conversation context.
	WindowSize int
	// MaxContextWords caps the context payload sent to the generator.
	MaxContextWords int
}

// reactions is sent as a standalone reply instead of generated text.
var reactions = []string{
	"😂", "🤡", "😏", "🔥", "💀", "😳", "🤌", "🙃", "😅", "😭",
	"кек", "лол", "хз", "жесть", "мда", "бывает", "хых", "кринж",
	"T_T", "...", "да ладно", "оно самое", "вот это поворот",
}

// helpText is the /help response.
const helpText = `Привет! Я Нейрода́н — генерирую текст по-русски и не только 🤖

Команды:
/generate [слова] — сгенерировать текст. Можно указать, с чего начать.
/anec [слова] — продолжить вашу фразу в духе анекдота.
/anec s [слова] — то же, но со стилизацией.
/help — показать эту справку.

Стилизация:
/on <фичи> — включить стилизацию.
/off <фичи> — выключить стилизацию.
/on или /off без аргументов — показать текущие настройки.

Доступные фичи: caps, allcaps, punct, emotes, swearify, intellectualize
Например: /off swearify caps`

// helpFeatures lists the stylization features for /on /off commands.
const helpFeatures = `Доступные фичи:
caps — КАПС в случайном слове
allcaps — фраза ЦЕЛИКОМ КАПСОМ
punct — пунктуация (…, !, !!)
emotes — реакции (😂, кек, лол)
swearify — мат
intellectualize — заумные слова

Например: /off swearify allcaps`

type Client struct {
	bot          *tgbotapi.BotAPI
	generatorURL string
	cfg          Config
	settings     *SettingsStore
	selfUsername string

	mu    sync.Mutex
	chats map[int64]*chatState
}

type trainRequest struct {
	Text   string `json:"text"`
	ChatID int64  `json:"chat_id"`
}

type generateRequest struct {
	LengthCfg string          `json:"length_cfg,omitempty"`
	Seed      string          `json:"seed,omitempty"`
	Context   string          `json:"context,omitempty"`
	Styling   *markov.Styling `json:"styling,omitempty"`
	Anecdote  bool            `json:"anecdote,omitempty"`
	ChatID    int64           `json:"chat_id"`
}

type generateResponse struct {
	Text string `json:"text"`
}

func NewClient(token, generatorURL string, cfg Config, settings *SettingsStore) *Client {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		panic(fmt.Sprintf("failed to create bot: %v", err))
	}

	c := &Client{
		bot:          bot,
		generatorURL: generatorURL,
		cfg:          cfg,
		settings:     settings,
		chats:        make(map[int64]*chatState),
	}
	if bot.Self.UserName != "" {
		c.selfUsername = bot.Self.UserName
	}
	return c
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

func (c *Client) handleCommand(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	args := strings.TrimSpace(msg.CommandArguments())
	switch msg.Command() {
	case "generate":
		c.generate(chatID, args)
	case "anec":
		c.anec(chatID, args)
	case "help":
		c.send(chatID, helpText)
	case "on":
		c.setFeatures(chatID, args, true)
	case "off":
		c.setFeatures(chatID, args, false)
	default:
		c.send(chatID, "Неизвестная команда. /help")
	}
}

// anec continues the user's text in the style of a joke ("анекдот").
//
// The command is un-stylized by default; a leading "s" flag
// ("/anec s [текст]") opts into the chat's stylization settings.
func (c *Client) anec(chatID int64, args string) {
	stylize, seed := parseAnecdoteArgs(args)

	var styling *markov.Styling
	if stylize {
		styling = c.stylingFor(chatID)
	}

	text := c.doGenerate(generateRequest{
		LengthCfg: "short",
		Seed:      seed,
		ChatID:    chatID,
		Styling:   styling,
		Anecdote:  true,
	})
	if text != "" {
		c.send(chatID, text)
	}
}

// parseAnecdoteArgs splits the raw /anec arguments into a stylize flag and
// the seed text. A leading "s" (case-insensitive) as the first token opts
// into stylization: "/anec s [текст]".
func parseAnecdoteArgs(args string) (stylize bool, seed string) {
	fields := strings.Fields(args)
	if len(fields) > 0 && strings.EqualFold(fields[0], "s") {
		stylize = true
		fields = fields[1:]
	}
	return stylize, strings.Join(fields, " ")
}

// setFeatures handles /on and /off: enables/disables stylization features.
func (c *Client) setFeatures(chatID int64, args string, enabled bool) {
	if strings.TrimSpace(args) == "" {
		c.send(chatID, c.settings.Summary(chatID))
		return
	}

	verb := "Включила"
	if !enabled {
		verb = "Выключила"
	}

	var done, unknown []string
	for _, raw := range strings.Fields(strings.ReplaceAll(args, ",", " ")) {
		f := normalizeFeature(raw)
		if !isValidFeature(f) {
			unknown = append(unknown, raw)
			continue
		}
		if err := c.settings.Set(chatID, f, enabled); err != nil {
			log.Printf("failed to save settings for chat %d: %v", chatID, err)
			c.send(chatID, "Не удалось сохранить настройки 😔")
			return
		}
		done = append(done, f)
	}

	var b strings.Builder
	if len(done) > 0 {
		fmt.Fprintf(&b, "%s: %s", verb, strings.Join(done, ", "))
	}
	if len(unknown) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "Не знаю такие фичи: %s\n\n%s", strings.Join(unknown, ", "), helpFeatures)
	}
	if b.Len() == 0 {
		b.WriteString("Не указана ни одна фича. /help")
	}
	c.send(chatID, b.String())
}

// isCommandText reports whether text looks like a bot command invocation
// ("/anec foo", "/start@MyBot") that should never be trained on.
func isCommandText(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "/")
}

// stylingFor builds the generator styling config from per-chat settings.
func (c *Client) stylingFor(chatID int64) *markov.Styling {
	return &markov.Styling{
		Caps:     c.settings.IsEnabled(chatID, FeatureCaps),
		AllCaps:  c.settings.IsEnabled(chatID, FeatureAllCaps),
		Punct:    c.settings.IsEnabled(chatID, FeaturePunct),
		Emotes:   c.settings.IsEnabled(chatID, FeatureEmotes),
		Swearify: c.settings.IsEnabled(chatID, FeatureSwear),
		Intel:    c.settings.IsEnabled(chatID, FeatureIntel),
	}
}

func (c *Client) handleMessage(msg *tgbotapi.Message) {
	if msg.Text == "" {
		return
	}
	chatID := msg.Chat.ID

	mentioned, clean := stripMention(msg.Text, c.selfUsername)
	if clean == "" {
		return
	}

	// Never learn from command invocations (incl. /anec). Commands are
	// normally routed to handleCommand, but in groups a command can arrive
	// here; command-like text is noise the bot shouldn't internalize.
	if isCommandText(clean) {
		return
	}

	state := c.stateFor(chatID)

	// The mention is never trained, but the rest of the message is.
	go c.train(chatID, clean)

	if !state.observeAndDecide(clean, mentioned, c.cfg) {
		return
	}

	go c.replyTo(chatID, state)
}

// replyTo sends one reply event: either a bare reaction or a chain of up to
// ChainMax generated messages, each seeded by the previous one.
func (c *Client) replyTo(chatID int64, state *chatState) {
	if state.randFloat64() < c.cfg.ReactionProb {
		c.send(chatID, reactions[state.randInt(len(reactions))])
		return
	}

	chainLen := 1 + state.randInt(c.cfg.ChainMax)
	contextText := state.context()
	for i := 0; i < chainLen; i++ {
		text := c.doGenerate(generateRequest{
			LengthCfg: c.cfg.Length,
			Context:   contextText,
			ChatID:    chatID,
			Styling:   c.stylingFor(chatID),
		})
		if text == "" {
			return
		}
		c.send(chatID, text)
		contextText = lastWords(contextText+" "+text, c.cfg.MaxContextWords)
		if i < chainLen-1 {
			time.Sleep(time.Second)
		}
	}
}

func (c *Client) stateFor(chatID int64) *chatState {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.chats[chatID]; ok {
		return s
	}
	s := newChatState()
	c.chats[chatID] = s
	return s
}

func (c *Client) mention() string {
	if c.selfUsername == "" {
		return ""
	}
	return "@" + c.selfUsername
}

func (c *Client) train(chatID int64, text string) {
	req := trainRequest{Text: text, ChatID: chatID}
	if err := c.post("/train", req, nil); err != nil {
		log.Printf("train error: %v", err)
	}
}

// generate handles the /generate command, seeding generation with the
// optional user-supplied words.
func (c *Client) generate(chatID int64, seed string) {
	text := c.doGenerate(generateRequest{
		LengthCfg: c.cfg.Length,
		Seed:      seed,
		ChatID:    chatID,
		Styling:   c.stylingFor(chatID),
	})
	if text != "" {
		c.send(chatID, text)
	}
}

// doGenerate posts a generate request and returns the generated text, or ""
// on error.
func (c *Client) doGenerate(req generateRequest) string {
	var resp generateResponse
	if err := c.post("/generate", req, &resp); err != nil {
		log.Printf("generate error: %v", err)
		return ""
	}
	return resp.Text
}

func (c *Client) send(chatID int64, text string) {
	reply := tgbotapi.NewMessage(chatID, text)
	if _, err := c.bot.Send(reply); err != nil {
		log.Printf("send error: %v", err)
	}
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

// stripMention removes the bot's @mention from text (case-insensitively) and
// reports whether the text was addressed to the bot. The returned text
// preserves the original case of everything except the removed mention.
func stripMention(text, username string) (bool, string) {
	if username == "" {
		return false, strings.TrimSpace(text)
	}
	mention := strings.ToLower(username)
	lower := strings.ToLower(text)
	if !strings.Contains(lower, mention) {
		return false, strings.TrimSpace(text)
	}

	// Walk the original text, skipping any occurrence of the (lowercased)
	// mention. mention is ASCII so byte-index alignment holds.
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); {
		if strings.HasPrefix(lower[i:], mention) {
			i += len(mention)
			b.WriteByte(' ')
		} else {
			b.WriteByte(text[i])
			i++
		}
	}
	// Drop a bare '@' left over from "@mention" and normalize whitespace.
	cleaned := strings.ReplaceAll(b.String(), "@ ", " ")
	return true, strings.Join(strings.Fields(cleaned), " ")
}

func lastWords(s string, n int) string {
	if n <= 0 {
		return s
	}
	words := strings.Fields(s)
	if len(words) <= n {
		return s
	}
	return strings.Join(words[len(words)-n:], " ")
}
