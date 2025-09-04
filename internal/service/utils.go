package service

import (
	"GoRoutine/internal/domain/entities"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

func FormatSegments(segments []entities.AudioSegment) string {
	var buf bytes.Buffer
	for _, s := range segments {
		buf.WriteString(fmt.Sprintf("%s: %s\n", s.Speaker, s.Text))
	}
	return buf.String()
}

func ExtractJSON(raw string) string {
	// Убираем блоки ```json ... ``` или ``` ... ```
	re := regexp.MustCompile("(?s)```json\\s*(.*?)\\s*```|```\\s*(.*?)\\s*```")
	match := re.FindStringSubmatch(raw)

	if len(match) >= 2 && match[1] != "" {
		return match[1]
	}
	if len(match) >= 3 && match[2] != "" {
		return match[2]
	}

	// Если совпадений нет, возвращаем как есть
	return raw
}

func SendToAI(apiURL, token, text string) (string, error) {
	reqBody := entities.ChatRequest{
		Temperature:      0.8,
		TopP:             0.95,
		MaxTokens:        4096,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
		Model:            "qwen2.572b",
		Messages: []entities.ChatMessage{
			{Role: "user", Content: text},
		},
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", err
	}

	// Получаем content из первого choice
	choices, ok := respData["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("no choices in AI response")
	}

	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid choice format")
	}

	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid message format")
	}

	content, ok := message["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is not string")
	}

	return content, nil
}
