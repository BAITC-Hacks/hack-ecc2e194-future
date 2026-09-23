package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hack-ecc2e194-future/internal/domain"
)

const openAIEndpoint = "https://api.openai.com/v1/chat/completions"
const openAIModel = "gpt-4o-mini"
const openAITimeout = 7 * time.Second

type openAIAnalyzer struct {
	apiKey string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// systemPrompt instructs the model to return structured JSON.
// AI must NOT add facts that the user did not provide.
const systemPrompt = `Ты бизнес-аналитик. Выдели отрасль и задай ровно 3 конкретных уточняющих вопроса к черновику задачи, закрывающих пробелы по данным, ожидаемому результату и метрикам. Не добавляй факты, которых нет в тексте. Формат ответа строго JSON: {"topic": string, "questions": [{"field": string, "text": string}]}`

func (o *openAIAnalyzer) Analyze(ctx context.Context, draft string) (domain.AnalysisResult, error) {
	req := chatRequest{Model: openAIModel}
	req.ResponseFormat.Type = "json_object"
	req.Messages = []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: draft},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return domain.AnalysisResult{}, fmt.Errorf("marshal openai request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, openAITimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpoint, bytes.NewReader(body))
	if err != nil {
		return domain.AnalysisResult{}, fmt.Errorf("build openai request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: openAITimeout}).Do(httpReq)
	if err != nil {
		return domain.AnalysisResult{}, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.AnalysisResult{}, fmt.Errorf("openai returned %s", resp.Status)
	}

	var completion chatResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&completion); err != nil {
		return domain.AnalysisResult{}, fmt.Errorf("decode openai response: %w", err)
	}
	if len(completion.Choices) == 0 {
		return domain.AnalysisResult{}, errors.New("openai returned no choices")
	}

	var result domain.AnalysisResult
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &result); err != nil {
		return domain.AnalysisResult{}, fmt.Errorf("parse openai json payload: %w", err)
	}
	if err := validateResult(result); err != nil {
		return domain.AnalysisResult{}, err
	}
	return result, nil
}

func validateResult(r domain.AnalysisResult) error {
	if strings.TrimSpace(r.Topic) == "" {
		return errors.New("openai response missing topic")
	}
	if len(r.Questions) != 3 {
		return fmt.Errorf("openai response has %d questions, want 3", len(r.Questions))
	}
	for i, q := range r.Questions {
		if strings.TrimSpace(q.Field) == "" || strings.TrimSpace(q.Text) == "" {
			return fmt.Errorf("openai question %d is incomplete", i)
		}
	}
	return nil
}
