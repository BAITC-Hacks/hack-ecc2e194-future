package ai

import (
	"context"
	"log"
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
	fallback := &heuristicAnalyzer{}
	if strings.TrimSpace(apiKey) != "" {
		return &fallbackAnalyzer{
			primary:  &openAIAnalyzer{apiKey: strings.TrimSpace(apiKey)},
			fallback: fallback,
		}
	}
	return fallback
}

type fallbackAnalyzer struct {
	primary  Analyzer
	fallback Analyzer
}

func (a *fallbackAnalyzer) Analyze(ctx context.Context, draft string) (domain.AnalysisResult, error) {
	result, err := a.primary.Analyze(ctx, draft)
	if err == nil {
		return result, nil
	}
	log.Printf("primary AI analysis failed; using heuristic fallback: %v", err)
	return a.fallback.Analyze(ctx, draft)
}
