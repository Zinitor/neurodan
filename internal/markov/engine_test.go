package markov_test

import (
	"strings"
	"testing"

	"github.com/aeremin/neurodan/internal/markov"
	"github.com/stretchr/testify/require"
)

func TestTrainExtractsTransitions(t *testing.T) {
	engine := markov.NewEngine()
	transitions := engine.Train("the cat sat on the mat")

	require.Len(t, transitions, 4)
	require.Equal(t, "the", transitions[0].Word1)
	require.Equal(t, "cat", transitions[0].Word2)
	require.Equal(t, "sat", transitions[0].NextWord)
	require.Equal(t, 1, transitions[0].Weight)
}

func TestTrainHandlesRepeatedText(t *testing.T) {
	engine := markov.NewEngine()
	trans := engine.Train("a b c")
	trans2 := engine.Train("a b c")

	require.Len(t, trans, 1)
	require.Len(t, trans2, 1)
	require.Equal(t, 1, trans[0].Weight)
	require.Equal(t, 1, trans2[0].Weight)
}

func TestGenerateReturnsText(t *testing.T) {
	engine := markov.NewEngine()

	transitions := []markov.Transition{
		{Word1: "the", Word2: "cat", NextWord: "sat", Weight: 2},
		{Word1: "cat", Word2: "sat", NextWord: "on", Weight: 2},
		{Word1: "sat", Word2: "on", NextWord: "the", Weight: 2},
		{Word1: "on", Word2: "the", NextWord: "mat", Weight: 2},
		{Word1: "the", Word2: "mat", NextWord: "the", Weight: 1},
	}

	result := engine.Generate(transitions, 5, "the")
	words := strings.Fields(result)

	require.GreaterOrEqual(t, len(words), 2)
	require.Equal(t, "the", words[0])
	require.Equal(t, "cat", words[1])
}

func TestGenerateWithEmptyTransitions(t *testing.T) {
	engine := markov.NewEngine()
	result := engine.Generate(nil, 10, "hello")
	require.Equal(t, "hello", result)
}
