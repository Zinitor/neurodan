package markov_test

import (
	"strings"
	"testing"

	"github.com/aeremin/neurodan/internal/markov"
	"github.com/stretchr/testify/require"
)

// trainAll is a helper that trains every joke into a single transition slice.
func trainAll(t *testing.T, jokes ...string) ([]markov.Transition, *markov.Engine) {
	t.Helper()
	engine := markov.NewEngine()
	var transitions []markov.Transition
	for _, j := range jokes {
		transitions = append(transitions, engine.Train(j)...)
	}
	return transitions, engine
}

// TestUnboundedContinuesPastLongGenPrefix is the regression test for the /anec
// echo bug: a genPrefix longer than the length preset target used to be echoed
// back verbatim because generation never attempted a continuation.
func TestUnboundedContinuesPastLongGenPrefix(t *testing.T) {
	jokes := []string{
		"шёл медведь по лесу видит машина горит",
		"шёл медведь по лесу видит заяц бежит",
		"шёл медведь по лесу видит лиса стоит",
		"встречаются два программиста в баре",
		"программист заходит в бар заказывает пиво",
	}
	transitions, engine := trainAll(t, jokes...)

	result := engine.Generate(transitions, markov.GenerateOptions{
		GenPrefix: "шёл медведь по лесу",
		Length:    markov.LengthUnbounded,
	})
	words := strings.Fields(result)
	require.Greater(t, len(words), 4, "expected continuation past the 4-word genPrefix, got %q", result)
	require.Equal(t, "шёл", words[0])
	require.Equal(t, "лесу", words[3])
}

// TestUnboundedNeverExceedsCap ensures the hard word ceiling is respected
// even when the graph is a single dense clique that could continue forever.
func TestUnboundedNeverExceedsCap(t *testing.T) {
	jokes := []string{
		"a b c d e f g h i j",
		"a c e g i k m o q s",
		"b d f h j l n p r t",
	}
	transitions, engine := trainAll(t, jokes...)

	result := engine.Generate(transitions, markov.GenerateOptions{
		GenPrefix: "a",
		Length:    markov.LengthUnbounded,
	})
	require.LessOrEqual(t, len(strings.Fields(result)), 400, "unbounded must still cap total words")
	require.NotEmpty(t, result)
}

// TestAnecLevel1PartialBridge is the "Шёл программист" case: the genPrefix's
// tail is unknown, but an earlier fragment bridges. The genPrefix is echoed and
// the continuation attaches at the known fragment.
func TestAnecLevel1PartialBridge(t *testing.T) {
	// "шёл" continues, "программист" is unknown.
	jokes := []string{
		"шёл как-то мужик по базару",
		"шёл дождь и морось",
		"шёл весёлый день",
	}
	transitions, engine := trainAll(t, jokes...)

	result := engine.Generate(transitions, markov.GenerateOptions{
		GenPrefix: "шёл программист",
		Length:    markov.LengthUnbounded,
	})
	words := strings.Fields(result)
	require.GreaterOrEqual(t, len(words), 2, "expected echo + continuation, got %q", result)
	require.Equal(t, "шёл", words[0])
	// The continuation must have appended at least one corpus word after the genPrefix.
	require.Greater(t, len(words), 2, "expected continuation past the genPrefix, got %q", result)
}

// TestAnecLevel2ColdOpen is the "динозавры в древнем риме" case: nothing in
// the genPrefix bridges at all. The genPrefix is echoed, then a fresh random
// anecdote is appended rather than failing or echoing.
func TestAnecLevel2ColdOpen(t *testing.T) {
	jokes := []string{
		"встречаются два программиста в баре",
		"программист заходит в бар заказывает пиво",
		"идут два программиста по улице",
	}
	transitions, engine := trainAll(t, jokes...)

	// Random made-up words so no n-gram (not even a function word) bridges.
	genPrefix := "квазидинозавры бормочут квантовый телескоп"
	result := engine.Generate(transitions, markov.GenerateOptions{
		GenPrefix: genPrefix,
		Length:    markov.LengthUnbounded,
	})
	require.NotEmpty(t, result, "must never return empty")
	require.Contains(t, result, "квазидинозавры", "genPrefix must be echoed")
	// There should be a fresh-joke boundary somewhere (newline separator).
	require.Contains(t, result, "\n", "expected cold-open separator, got %q", result)
}

// TestAnecEmptyGenPrefixStillGenerates ensures /anec with no genPrefix
// produces a random anecdote rather than an empty string.
func TestAnecEmptyGenPrefixStillGenerates(t *testing.T) {
	jokes := []string{
		"встречаются два программиста в баре",
		"программист заходит в бар заказывает пиво",
	}
	transitions, engine := trainAll(t, jokes...)

	result := engine.Generate(transitions, markov.GenerateOptions{
		Length: markov.LengthUnbounded,
	})
	require.NotEmpty(t, result)
}
