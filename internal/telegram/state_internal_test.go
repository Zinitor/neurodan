package telegram

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStripMention(t *testing.T) {
	mentioned, clean := stripMention("привет @testBot как дела", "testbot")
	require.True(t, mentioned)
	require.Equal(t, "привет как дела", clean)
}

func TestStripMentionCaseInsensitive(t *testing.T) {
	mentioned, clean := stripMention("Привет @TESTbot", "testBot")
	require.True(t, mentioned)
	require.Equal(t, "Привет", clean)
}

func TestStripMentionNoUsername(t *testing.T) {
	mentioned, clean := stripMention("просто сообщение", "")
	require.False(t, mentioned)
	require.Equal(t, "просто сообщение", clean)
}

func TestStripMentionAbsent(t *testing.T) {
	mentioned, clean := stripMention("просто сообщение", "somebot")
	require.False(t, mentioned)
	require.Equal(t, "просто сообщение", clean)
}

func TestStripMentionOnlyMention(t *testing.T) {
	mentioned, clean := stripMention("@testbot", "testbot")
	require.True(t, mentioned)
	require.Empty(t, clean)
}

func TestStripMentionPreservesCase(t *testing.T) {
	mentioned, clean := stripMention("Привет @TestBot КАК Дела", "testbot")
	require.True(t, mentioned)
	require.Equal(t, "Привет КАК Дела", clean)
}

func TestStripMentionMultipleOccurrences(t *testing.T) {
	mentioned, clean := stripMention("@testbot смотри @testbot сюда", "testbot")
	require.True(t, mentioned)
	require.Equal(t, "смотри сюда", clean)
}

func TestLastWords(t *testing.T) {
	// n <= 0 means "no truncation".
	require.Equal(t, "a b c", lastWords("a b c", 0))
	require.Equal(t, "a b c", lastWords("a b c", 10))
	require.Equal(t, "b c d", lastWords("a b c d", 3))
}

// TestStackingProbabilityAlwaysRepliesAtMax forces the probability to 1.0 by
// setting base above 1.0, so every message must trigger a reply.
func TestStackingProbabilityAlwaysReplies(t *testing.T) {
	cfg := Config{ReplyBase: 2.0, ReplyGrowth: 0, ReplyMax: 1.0, WindowSize: 10}
	state := newChatState()
	for i := 0; i < 100; i++ {
		require.True(t, state.observeAndDecide("привет", false, cfg), "must reply on iteration %d", i)
	}
}

// TestStackingProbabilityNeverReplies forces probability to 0 so the counter
// must keep growing and every message is skipped.
func TestStackingProbabilityNeverReplies(t *testing.T) {
	cfg := Config{ReplyBase: 0, ReplyGrowth: 0, ReplyMax: 0, WindowSize: 10}
	state := newChatState()
	for i := 0; i < 100; i++ {
		require.False(t, state.observeAndDecide("привет", false, cfg))
	}
}

// TestStackingProbabilityMentionForcesReply verifies that even with zero
// probability a mention triggers a reply and resets the window counter.
func TestStackingProbabilityMentionForcesReply(t *testing.T) {
	cfg := Config{ReplyBase: 0, ReplyGrowth: 0, ReplyMax: 0, WindowSize: 10}
	state := newChatState()
	for i := 0; i < 10; i++ {
		state.observeAndDecide("тишина", false, cfg)
	}
	require.True(t, state.observeAndDecide("смотри сюда", true, cfg))
	// After a mention reply, the counter is reset, so the next (unlucky)
	// message goes back to silence.
	require.False(t, state.observeAndDecide("снова тишина", false, cfg))
}

// TestWindowBounds checks the conversation window never grows beyond the
// configured size and that context() reflects the most recent messages.
func TestWindowBounds(t *testing.T) {
	cfg := Config{ReplyBase: 2.0, ReplyGrowth: 0, ReplyMax: 1.0, WindowSize: 3}
	state := newChatState()
	for i := 0; i < 10; i++ {
		state.observeAndDecide("msg", false, cfg)
	}
	require.Len(t, state.messages, 3)
	require.Equal(t, "msg msg msg", state.context())
}
