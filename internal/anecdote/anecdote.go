// Package anecdote builds the Markov model that powers the /anec command.
//
// The model is a single precomputed markov.Index over the whole joke corpus,
// built once at startup (or precomputed to a file) and then shared by every
// /anec request. It never touches chat data: each request just bridges the
// user's seed into this shared model, and the cold-open fallback guarantees
// a response even for a fully out-of-vocabulary seed.
package anecdote

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/aeremin/neurodan/internal/markov"
)

// LoadJokes reads a newline-separated file of jokes, skipping blank lines.
func LoadJokes(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open corpus %s: %w", path, err)
	}
	defer f.Close()

	var jokes []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			jokes = append(jokes, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read corpus %s: %w", path, err)
	}
	return jokes, nil
}

// BuildIndex trains every joke in the corpus into a single precomputed index.
func BuildIndex(jokes []string) *markov.Index {
	engine := markov.NewEngine()
	var transitions []markov.Transition
	for _, joke := range jokes {
		transitions = append(transitions, engine.Train(joke)...)
	}
	return markov.NewIndex(transitions)
}
