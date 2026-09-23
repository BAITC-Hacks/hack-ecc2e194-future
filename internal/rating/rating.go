package rating

import (
	"strings"

	"hack-ecc2e194-future/internal/domain"
)

type fieldCheck struct {
	key       string
	value     string
	minLength int
	points    int
}

// Calculate returns (score 0–100, readiness level, list of missing field keys).
// It is a pure function with no I/O dependencies.
func Calculate(f domain.TaskFields) (score int, level string, missing []string) {
	missing = make([]string, 0)

	checks := []fieldCheck{
		{"context_and_need", f.ContextAndNeed, 40, 20},
		{"data_and_materials", f.DataAndMaterials, 20, 20},
		{"expected_result", f.ExpectedResult, 25, 15},
		{"success_criteria", f.SuccessCriteria, 20, 15},
		{"constraints", f.Constraints, 15, 10},
		{"target_users", f.TargetUsers, 10, 10},
	}

	for _, c := range checks {
		if len([]rune(strings.TrimSpace(c.value))) >= c.minLength {
			score += c.points
		} else {
			missing = append(missing, c.key)
		}
	}

	contact := f.ContactAndFeedback
	if strings.Contains(contact, "@") || strings.Contains(contact, "http") {
		score += 10
	} else {
		missing = append(missing, "contact_and_feedback")
	}

	switch {
	case score >= 90:
		level = "PRIORITY"
	case score >= 70:
		level = "READY"
	case score >= 40:
		level = "IN_PROGRESS"
	default:
		level = "DRAFT"
	}
	return
}
