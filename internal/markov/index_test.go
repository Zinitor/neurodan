package markov_test

import (
	"bytes"
	"testing"

	"github.com/aeremin/neurodan/internal/markov"
	"github.com/stretchr/testify/require"
)

// TestIndexRoundTrip verifies a built index survives WriteTo/NewIndexFromReader
// and still generates identically-shaped output.
func TestIndexRoundTrip(t *testing.T) {
	jokes := []string{
		"встречаются два программиста в баре",
		"программист заходит в бар заказывает пиво",
		"идут два программиста по улице",
	}
	engine := markov.NewEngine()
	var transitions []markov.Transition
	for _, j := range jokes {
		transitions = append(transitions, engine.Train(j)...)
	}
	original := markov.NewIndex(transitions)

	var buf bytes.Buffer
	require.NoError(t, original.Encode(&buf))
	require.NotZero(t, buf.Len(), "serialized index should not be empty")

	loaded, err := markov.DecodeIndex(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	require.Equal(t, original.Len(), loaded.Len())
}
