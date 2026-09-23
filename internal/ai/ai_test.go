package ai

import (
	"context"
	"errors"
	"testing"

	"hack-ecc2e194-future/internal/domain"
)

type analyzerFunc func(context.Context, string) (domain.AnalysisResult, error)

func (f analyzerFunc) Analyze(ctx context.Context, draft string) (domain.AnalysisResult, error) {
	return f(ctx, draft)
}

func TestFallbackAnalyzerUsesHeuristicAfterPrimaryError(t *testing.T) {
	fallbackCalled := false
	want := domain.AnalysisResult{Topic: "Логистика"}
	analyzer := &fallbackAnalyzer{
		primary: analyzerFunc(func(context.Context, string) (domain.AnalysisResult, error) {
			return domain.AnalysisResult{}, errors.New("OpenAI unavailable")
		}),
		fallback: analyzerFunc(func(_ context.Context, draft string) (domain.AnalysisResult, error) {
			fallbackCalled = true
			if draft != "задача про доставку" {
				t.Fatalf("fallback draft = %q, want original draft", draft)
			}
			return want, nil
		}),
	}

	got, err := analyzer.Analyze(context.Background(), "задача про доставку")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if !fallbackCalled {
		t.Fatal("fallback analyzer was not called")
	}
	if got.Topic != want.Topic {
		t.Fatalf("topic = %q, want %q", got.Topic, want.Topic)
	}
}

func TestNewWithoutAPIKeyUsesHeuristic(t *testing.T) {
	result, err := New("").Analyze(context.Background(), "Нужна оптимизация маршрутов доставки")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if result.Topic != "Логистика" {
		t.Fatalf("topic = %q, want Логистика", result.Topic)
	}
	if len(result.Questions) != 3 {
		t.Fatalf("questions = %d, want 3", len(result.Questions))
	}
}
