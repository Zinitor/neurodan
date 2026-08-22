package telegram

import (
	"math/rand"
	"strings"
	"sync"
	"time"
)

// chatState tracks per-chat conversation state. The zero value is not usable;
// use newChatState.
type chatState struct {
	mu                 sync.Mutex
	messages           []string
	messagesSinceReply int
	rng                *rand.Rand
}

func newChatState() *chatState {
	return &chatState{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// observeAndDecide records text into the conversation window and decides
// whether the bot should reply, using a stacking probability that grows the
// longer the bot stays silent.
//
// mentioned forces a reply (reset the counter) regardless of probability.
func (s *chatState) observeAndDecide(text string, mentioned bool, cfg Config) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, text)
	if cfg.WindowSize > 0 && len(s.messages) > cfg.WindowSize {
		s.messages = append([]string(nil), s.messages[len(s.messages)-cfg.WindowSize:]...)
	}

	if mentioned {
		s.messagesSinceReply = 0
		return true
	}

	p := cfg.ReplyBase + cfg.ReplyGrowth*float64(s.messagesSinceReply)
	if p > cfg.ReplyMax {
		p = cfg.ReplyMax
	}

	if s.rng.Float64() < p {
		s.messagesSinceReply = 0
		return true
	}
	s.messagesSinceReply++
	return false
}

// context returns the recent conversation window joined into a single string.
func (s *chatState) context() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.messages, " ")
}

func (s *chatState) randFloat64() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64()
}

func (s *chatState) randInt(n int) int {
	if n <= 0 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Intn(n)
}
