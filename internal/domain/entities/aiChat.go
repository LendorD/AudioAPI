package entities

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Seed             *int          `json:"seed"`
	Stop             *string       `json:"stop"`
	Temperature      float64       `json:"temperature"`
	TopP             float64       `json:"top_p"`
	MaxTokens        int           `json:"max_tokens"`
	FrequencyPenalty int           `json:"frequency_penalty"`
	PresencePenalty  int           `json:"presence_penalty"`
	Model            string        `json:"model"`
	Messages         []ChatMessage `json:"messages"`
}

type AIResult struct {
	Theme           string  `json:"theme"`
	Deal            string  `json:"deal"`
	DealDescription string  `json:"deal_description"`
	CompleteDeal    bool    `json:"complete_deal"`
	DealPrice       float64 `json:"deal_price"`
}
