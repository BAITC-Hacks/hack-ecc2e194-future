package ai

import (
	"context"
	"strings"

	"hack-ecc2e194-future/internal/domain"
)

// Analyzer analyses a draft task description and returns clarifying questions.
type Analyzer interface {
	Analyze(ctx context.Context, draft string) (domain.AnalysisResult, error)
}

// New returns an OpenAI-backed Analyzer when apiKey is non-empty,
// otherwise it returns the heuristic fallback.
func New(apiKey string) Analyzer {
	if strings.TrimSpace(apiKey) != "" {
		return &openAIAnalyzer{apiKey: strings.TrimSpace(apiKey)}
	}
	return &heuristicAnalyzer{}
}
