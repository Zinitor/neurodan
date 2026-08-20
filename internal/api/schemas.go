package api

type TrainRequest struct {
	Text   string `json:"text"`
	ChatID int64  `json:"chat_id"`
}

type GenerateRequest struct {
	Length int    `json:"length"`
	Seed   string `json:"seed,omitempty"`
	ChatID int64  `json:"chat_id"`
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
