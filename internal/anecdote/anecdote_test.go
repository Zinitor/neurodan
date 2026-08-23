package anecdote_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aeremin/neurodan/internal/anecdote"
	"github.com/stretchr/testify/require"
)

func writeJokes(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jokes.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadJokesSkipsBlanks(t *testing.T) {
	path := writeJokes(t, "первый анекдот\n\nвторой анекдот\n   \nтретий\n")
	jokes, err := anecdote.LoadJokes(path)
	require.NoError(t, err)
	require.Equal(t, []string{"первый анекдот", "второй анекдот", "третий"}, jokes)
}

func TestLoadJokesMissingFile(t *testing.T) {
	_, err := anecdote.LoadJokes(filepath.Join(t.TempDir(), "nope.txt"))
	require.Error(t, err)
}

func TestBuildIndexTrainsAllJokes(t *testing.T) {
	jokes := []string{"шёл медведь по лесу", "шёл дождь и морось"}
	idx := anecdote.BuildIndex(jokes)
	// 5 transitions per 4-word joke: (n-1)*orders.
	require.Equal(t, 12, idx.Len())
}

func TestBuildIndexEmptyCorpus(t *testing.T) {
	idx := anecdote.BuildIndex(nil)
	require.NotNil(t, idx)
	require.Equal(t, 0, idx.Len())
}
