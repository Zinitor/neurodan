package markov

import (
	"math/rand"
	"strings"
)

type Transition struct {
	Word1    string
	Word2    string
	NextWord string
	Weight   int
}

type Engine struct {
	rng *rand.Rand
}

func NewEngine() *Engine {
	return &Engine{rng: rand.New(rand.NewSource(rand.Int63()))}
}

func (e *Engine) Train(text string) []Transition {
	words := strings.Fields(text)
	if len(words) < 3 {
		return nil
	}

	var transitions []Transition
	for i := 0; i <= len(words)-3; i++ {
		transitions = append(transitions, Transition{
			Word1:    words[i],
			Word2:    words[i+1],
			NextWord: words[i+2],
			Weight:   1,
		})
	}
	return transitions
}

func (e *Engine) Generate(transitions []Transition, length int, seed string) string {
	if len(transitions) == 0 {
		return seed
	}

	words := strings.Fields(seed)
	if len(words) < 2 {
		t := transitions[e.rng.Intn(len(transitions))]
		words = []string{t.Word1, t.Word2}
	}

	result := make([]string, len(words))
	copy(result, words)

	for len(result) < length+2 {
		w1 := result[len(result)-2]
		w2 := result[len(result)-1]

		var candidates []Transition
		for _, t := range transitions {
			if t.Word1 == w1 && t.Word2 == w2 {
				candidates = append(candidates, t)
			}
		}
		if len(candidates) == 0 {
			break
		}

		totalWeight := 0
		for _, c := range candidates {
			totalWeight += c.Weight
		}

		r := e.rng.Intn(totalWeight)
		for _, c := range candidates {
			r -= c.Weight
			if r < 0 {
				result = append(result, c.NextWord)
				break
			}
		}
	}

	return strings.Join(result, " ")
}
