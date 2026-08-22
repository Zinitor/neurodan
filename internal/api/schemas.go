package api

import "github.com/aeremin/neurodan/internal/markov"

type TrainRequest struct {
	Text   string `json:"text"`
	ChatID int64  `json:"chat_id"`
}

type GenerateRequest struct {
	// Length is kept for backwards compatibility. Prefer LengthCfg.
	Length int `json:"length,omitempty"`
	// LengthCfg selects the length preset: short, medium or long.
	LengthCfg string `json:"length_cfg,omitempty"`
	// Seed forces generation to start with these words.
	Seed string `json:"seed,omitempty"`
	// Context is recent conversation text. When Seed is empty, the longest
	// suffix of Context that appears in the model seeds generation.
	Context string `json:"context,omitempty"`
	// Styling controls which stylization passes run. Nil means defaults.
	Styling *markov.Styling `json:"styling,omitempty"`
	ChatID  int64           `json:"chat_id"`
}

type GenerateResponse struct {
	Text string `json:"text"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
