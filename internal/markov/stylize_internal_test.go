package markov

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStylizePreservesWords hammers the probabilistic branches of stylize
// with a seeded RNG and verifies no words are ever lost or reordered.
func TestStylizePreservesWords(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	const input = "привет мир это тест"
	expected := strings.Fields(input)
	for i := 0; i < 2000; i++ {
		out := stylize(input, DefaultStyling(), r)
		for _, w := range expected {
			require.True(t, strings.Contains(strings.ToLower(out), strings.ToLower(w)),
				"lost word %q in %q", w, out)
		}
	}
}

// TestSwearifyPreservesWordsAndOrder verifies swearify never drops or
// reorders the original words, regardless of which injection slots fire.
func TestSwearifyPreservesWordsAndOrder(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	expected := []string{"я", "закончил", "кушать", "суп"}
	for i := 0; i < 5000; i++ {
		out := swearify("Я закончил кушать суп", r)
		lower := strings.ToLower(out)
		last := -1
		for _, w := range expected {
			idx := strings.Index(lower, w)
			require.Greater(t, idx, last, "word %q out of order or missing in %q", w, out)
			last = idx
		}
	}
}

// TestSwearifyPreservesTrailingPunctuation verifies that injected swears
// always land before trailing punctuation: "... кушать нахуй!" never
// "... кушать! нахуй".
func TestSwearifyPreservesTrailingPunctuation(t *testing.T) {
	r := rand.New(rand.NewSource(123))
	for i := 0; i < 5000; i++ {
		out := swearify("Я закончил кушать!", r)
		require.True(t, strings.HasSuffix(out, "!"), "lost trailing punctuation in %q", out)
		require.False(t, strings.Contains(out, "! "), "text after punctuation in %q", out)
	}
}

func TestSwearifyHandlesEmpty(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	require.Equal(t, "", swearify("", r))
	require.Equal(t, "   ", swearify("   ", r))
}

// TestIntellectualizePreservesWordsAndOrder is the high-register counterpart
// to TestSwearifyPreservesWordsAndOrder.
func TestIntellectualizePreservesWordsAndOrder(t *testing.T) {
	r := rand.New(rand.NewSource(99))
	expected := []string{"я", "закончил", "кушать", "суп"}
	for i := 0; i < 5000; i++ {
		out := intellectualize("Я закончил кушать суп", r)
		lower := strings.ToLower(out)
		last := -1
		for _, w := range expected {
			idx := strings.Index(lower, w)
			require.Greater(t, idx, last, "word %q out of order or missing in %q", w, out)
			last = idx
		}
	}
}

// TestIntellectualizePreservesTrailingPunctuation mirrors the swear version.
func TestIntellectualizePreservesTrailingPunctuation(t *testing.T) {
	r := rand.New(rand.NewSource(123))
	for i := 0; i < 5000; i++ {
		out := intellectualize("Я закончил кушать!", r)
		require.True(t, strings.HasSuffix(out, "!"), "lost trailing punctuation in %q", out)
		require.False(t, strings.Contains(out, "! "), "text after punctuation in %q", out)
	}
}

func TestIntellectualizeHandlesEmpty(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	require.Equal(t, "", intellectualize("", r))
	require.Equal(t, "   ", intellectualize("   ", r))
}

// TestRegistersMix verifies both registers can appear in the same message
// over enough runs: swears and intellectual filler should both show up, and
// occasionally together (the "системно ... бля" effect).
func TestRegistersMix(t *testing.T) {
	r := rand.New(rand.NewSource(2024))
	sawSwear := false
	sawIQ := false
	for i := 0; i < 20000; i++ {
		out := stylize("Я закончил кушать суп", DefaultStyling(), r)
		if containsAny(out, swearInfix, swearTrailing) {
			sawSwear = true
		}
		if containsAny(out, iqInfix, iqTrailing) {
			sawIQ = true
		}
		if sawSwear && sawIQ {
			break
		}
	}
	require.True(t, sawSwear, "expected swearing to appear at least once")
	require.True(t, sawIQ, "expected intellectual filler to appear at least once")
}

// TestRegistersCanCoexistInOneMessage directly asserts the mix is possible:
// with enough iterations we should see at least one message carrying both
// registers.
func TestRegistersCanCoexistInOneMessage(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 200000; i++ {
		out := stylize("Я закончил кушать суп", DefaultStyling(), r)
		if containsAny(out, swearInfix, swearTrailing) && containsAny(out, iqInfix, iqTrailing) {
			return // found a mix
		}
	}
	t.Fatal("expected at least one message with both registers mixed in 200k runs")
}

func TestIntellectualizeInjectsSometimes(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	injected := 0
	const iterations = 20000
	for i := 0; i < iterations; i++ {
		out := intellectualize("Я закончил кушать суп", r)
		if out != "Я закончил кушать суп" {
			injected++
		}
	}
	// At least one slot is ~49% (1 - 0.8*0.85*0.75) — make sure we see it.
	require.Greater(t, injected, 0)
}

func containsAny(s string, lists ...[]string) bool {
	for _, list := range lists {
		for _, w := range list {
			if strings.Contains(s, w) {
				return true
			}
		}
	}
	return false
}

func TestInsertWord(t *testing.T) {
	words := []string{"a", "b", "c"}
	got := insertWord(words, 1, "X")
	require.Equal(t, []string{"a", "X", "b", "c"}, got)

	got = insertWord([]string{"a", "b", "c", "d"}, 3, "X")
	require.Equal(t, []string{"a", "b", "c", "X", "d"}, got)
}
