package ai

import (
	"context"
	"strings"

	"hack-ecc2e194-future/internal/domain"
)

type heuristicAnalyzer struct{}

// Analyze detects the industry from keywords and produces 3 contextual questions
// based on which information blocks are absent from the draft.
func (h *heuristicAnalyzer) Analyze(_ context.Context, draft string) (domain.AnalysisResult, error) {
	lower := strings.ToLower(draft)
	return domain.AnalysisResult{
		Topic:     detectTopic(lower),
		Questions: selectQuestions(lower),
	}, nil
}

func detectTopic(lower string) string {
	topics := []struct {
		name     string
		keywords []string
	}{
		{"Финансы", []string{"финанс", "банк", "кредит", "платёж", "оплат", "страхован"}},
		{"Логистика", []string{"логистик", "доставк", "маршрут", "склад", "груз", "курьер"}},
		{"Образование", []string{"образован", "студент", "курс", "обучен", "школ", "университет"}},
		{"Медицина", []string{"медицин", "здоров", "пациент", "клиник", "врач", "диагноз"}},
		{"Экология", []string{"экологи", "энерги", "выброс", "зелен", "климат", "воздух"}},
		{"Ритейл", []string{"торговл", "магазин", "продаж", "ритейл", "товар", "заказ"}},
		{"Производство", []string{"производств", "завод", "фабрик", "оборудован", "станок"}},
		{"HR / Кадры", []string{"сотрудник", "персонал", "найм", "вакансия", "кандидат"}},
	}
	for _, t := range topics {
		for _, kw := range t.keywords {
			if strings.Contains(lower, kw) {
				return t.name
			}
		}
	}
	return "Общий"
}

// questionCandidate describes a potential clarifying question with a
// prerequisite check: if the draft already mentions keywords from `present`,
// the question is skipped.
type questionCandidate struct {
	field   string
	text    string
	present []string
}

var allQuestions = []questionCandidate{
	{
		field:   "data_and_materials",
		text:    "Какие данные, файлы или API будут предоставлены команде для работы над задачей?",
		present: []string{"данн", "dataset", "api", "таблиц", "файл", "база данных"},
	},
	{
		field:   "expected_result",
		text:    "Какой конкретный артефакт должна передать команда по итогам работы (прототип, отчёт, модель, демо)?",
		present: []string{"результат", "прототип", "продукт", "решени", "приложен", "артефакт"},
	},
	{
		field:   "success_criteria",
		text:    "По каким измеримым критериям бизнес определит, что задача решена успешно?",
		present: []string{"метрик", "kpi", "критерий", "критери", "измер", "процент", "точност"},
	},
	{
		field:   "target_users",
		text:    "Кто является конечным пользователем решения и каковы его главные потребности?",
		present: []string{"пользовател", "для кого", "аудитор", "целевой"},
	},
	{
		field:   "constraints",
		text:    "Какие есть ограничения по технологиям, срокам или доступам для работы над задачей?",
		present: []string{"ограничен", "срок", "стек", "технолог", "бюджет"},
	},
	{
		field:   "contact_and_feedback",
		text:    "Как команда сможет связаться с вами для уточнений во время работы?",
		present: []string{"@", "http", "контакт", "связ", "почт", "telegram"},
	},
}

func selectQuestions(lower string) []domain.DraftQuestion {
	result := make([]domain.DraftQuestion, 0, 3)
	for _, c := range allQuestions {
		if len(result) == 3 {
			break
		}
		if !containsAny(lower, c.present) {
			result = append(result, domain.DraftQuestion{Field: c.field, Text: c.text})
		}
	}
	// Guarantee exactly 3 questions by cycling through all candidates again.
	for _, c := range allQuestions {
		if len(result) == 3 {
			break
		}
		// Add only if not already included.
		already := false
		for _, q := range result {
			if q.Field == c.field {
				already = true
				break
			}
		}
		if !already {
			result = append(result, domain.DraftQuestion{Field: c.field, Text: c.text})
		}
	}
	return result[:3]
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
