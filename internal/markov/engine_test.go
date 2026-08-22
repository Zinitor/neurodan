package markov_test

import (
	"strings"
	"testing"

	"github.com/aeremin/neurodan/internal/markov"
	"github.com/stretchr/testify/require"
)

func TestTrainEmitsAllOrders(t *testing.T) {
	engine := markov.NewEngine()
	transitions := engine.Train("the cat sat on the mat")

	// Orders for "the cat sat on the mat" (6 words):
	// order1: (the)->cat (cat)->sat (sat)->on (on)->the (the)->mat   = 5
	// order2: (the,cat)->sat (cat,sat)->on (sat,on)->the (on,the)->mat = 4
	// order3: (the,cat,sat)->on (cat,sat,on)->the (sat,on,the)->mat   = 3
	require.Len(t, transitions, 12)

	byOrder := map[int]int{}
	for _, tr := range transitions {
		require.Equal(t, tr.Order, len(tr.Prefix))
		require.NotEmpty(t, tr.NextWord)
		byOrder[tr.Order]++
	}
	require.Equal(t, 5, byOrder[1])
	require.Equal(t, 4, byOrder[2])
	require.Equal(t, 3, byOrder[3])
}

func TestTrainShortText(t *testing.T) {
	engine := markov.NewEngine()
	require.Nil(t, engine.Train(""))
	require.Nil(t, engine.Train("one"))
	require.Len(t, engine.Train("one two"), 1) // only (one)->two
}

func TestGenerateStartsFromSeed(t *testing.T) {
	engine := markov.NewEngine()
	var transitions []markov.Transition
	for _, word := range []string{"a b c", "a b d", "a b e", "a b f", "a b g", "a b h", "a b i", "a b j"} {
		transitions = append(transitions, engine.Train(word)...)
	}

	result := engine.Generate(transitions, markov.GenerateOptions{
		Seed:   "a b",
		Length: markov.LengthShort,
	})
	words := strings.Fields(result)
	require.GreaterOrEqual(t, len(words), 2)
	require.Equal(t, "a", words[0])
	require.Equal(t, "b", words[1])
}

func TestGenerateWithEmptyTransitions(t *testing.T) {
	engine := markov.NewEngine()
	result := engine.Generate(nil, markov.GenerateOptions{Seed: "hello"})
	require.Equal(t, "", result)
}

func TestGenerateSelfInitiatesWithoutSeed(t *testing.T) {
	engine := markov.NewEngine()
	var transitions []markov.Transition
	for _, s := range []string{"a b c", "a b d", "a b e", "a b f", "x y z", "x y w"} {
		transitions = append(transitions, engine.Train(s)...)
	}

	// No seed, no context: the engine must still return something by starting
	// from a random prefix.
	result := engine.Generate(transitions, markov.GenerateOptions{Length: markov.LengthShort})
	require.NotEmpty(t, result)
	vocab := map[string]bool{"a": true, "b": true, "c": true, "d": true, "e": true, "f": true, "x": true, "y": true, "z": true, "w": true}
	for _, w := range strings.Fields(result) {
		require.True(t, vocab[w], "unexpected word %q in %q", w, result)
	}
}

func TestGenerateRespectsLengthPreset(t *testing.T) {
	engine := markov.NewEngine()
	// Dense graph so generation has plenty to walk through.
	corpus := []string{
		"один два три четыре пять", "пять четыре три два один",
		"раз два три четыре пять", "пять четыре три два раз",
		"котик бежит по полю", "поле зелёное а небо синее",
		"собака гавкает на луну", "луна круглая как сыр",
	}
	var transitions []markov.Transition
	for _, s := range corpus {
		transitions = append(transitions, engine.Train(s)...)
	}

	short := engine.Generate(transitions, markov.GenerateOptions{Seed: "один два", Length: markov.LengthShort})
	require.LessOrEqual(t, len(strings.Fields(short)), 5, "short preset should stay tiny, got %q", short)
	require.GreaterOrEqual(t, len(short), 1)

	long := engine.Generate(transitions, markov.GenerateOptions{Seed: "один два", Length: markov.LengthLong})
	require.GreaterOrEqual(t, len(strings.Fields(long)), 2)
}

func TestBridgePicksUpContext(t *testing.T) {
	engine := markov.NewEngine()
	var transitions []markov.Transition
	for _, s := range []string{"a b c", "a b d", "a b e", "b c x", "b c y", "b c z", "a b c x", "a b c y"} {
		transitions = append(transitions, engine.Train(s)...)
	}

	// "y z" is not a prefix, "z" is not a prefix; but "b c" is a suffix of
	// "a b c" -> longest suffix that is a prefix is "b c".
	seed := engine.Bridge(transitions, "x y b c")
	require.Equal(t, "b c", seed)
}

func TestBridgeReturnsEmptyWhenNothingMatches(t *testing.T) {
	engine := markov.NewEngine()
	var transitions []markov.Transition
	for _, s := range []string{"a b c", "a b d"} {
		transitions = append(transitions, engine.Train(s)...)
	}
	require.Equal(t, "", engine.Bridge(transitions, "zzz yyy xxx"))
	require.Equal(t, "", engine.Bridge(transitions, ""))
}

func TestStylizeHandlesTinyInput(t *testing.T) {
	engine := markov.NewEngine()
	// Stylize must never drop content, even for a single word.
	require.True(t, strings.Contains(engine.Stylize("x", markov.DefaultStyling()), "x"))
	require.Equal(t, "", engine.Stylize("", markov.DefaultStyling()))
}

// TestStylizeDisabledIsIdentity verifies the zero Styling value disables
// every pass, leaving the words (modulo whitespace normalization) untouched.
func TestStylizeDisabledIsIdentity(t *testing.T) {
	engine := markov.NewEngine()
	result := engine.Stylize("Я закончил кушать", markov.Styling{})
	require.Equal(t, "Я закончил кушать", result)
}
