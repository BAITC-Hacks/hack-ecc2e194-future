package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"

	"hack-ecc2e194-future/internal/domain"
	"hack-ecc2e194-future/internal/rating"
)

// Seed populates the database with demo tasks, teams and proposals
// if no tasks exist yet. Safe to call on every startup.
func Seed(db *sql.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	tasks := []domain.Task{
		{
			ID: "task-1", Title: "Прогноз спроса на основе истории продаж", Topic: "Ритейл",
			Status: "open",
			Fields: domain.TaskFields{
				ContextAndNeed: "Компания теряет до 15% выручки из-за ошибочных заказов: часть товаров замораживается на складе, другие постоянно заканчиваются. Нам нужна система прогнозирования спроса.",
			},
		},
		{
			ID: "task-2", Title: "Оптимизация маршрутов доставки последней мили", Topic: "Логистика",
			Status: "open",
			Fields: domain.TaskFields{
				ContextAndNeed:   "Курьеры тратят до 40% времени в пробках и на неоптимальных маршрутах. Хотим сократить операционные расходы и время доставки.",
				DataAndMaterials: "CSV с историей заказов за 12 месяцев (координаты, время доставки, вес посылки).",
				TargetUsers:      "Диспетчеры и курьеры службы доставки.",
			},
		},
		{
			ID: "task-3", Title: "Анализ энергопотребления зданий", Topic: "Экология",
			Status: "open",
			Fields: domain.TaskFields{
				ContextAndNeed:     "Управляем портфелем из 30 офисных зданий. Расходы на электроэнергию выросли на 22% за год, но мы не понимаем причин и не можем выявить аномалии потребления.",
				DataAndMaterials:   "API СКУД со счётчиками каждые 15 минут, данные метеостанции. История за 2 года.",
				ExpectedResult:     "Дашборд с аномалиями и рекомендациями по снижению потребления.",
				SuccessCriteria:    "Снижение выявленных аномалий на 30%, точность модели > 85%.",
				Constraints:        "Стек: Python или JS. Срок — 5 часов хакатона.",
				TargetUsers:        "Facility-менеджеры зданий.",
				ContactAndFeedback: "mailto:energy@example.com",
			},
		},
		{
			ID: "task-4", Title: "Система рекомендаций образовательных курсов", Topic: "Образование",
			Status: "open",
			Fields: domain.TaskFields{
				ContextAndNeed:     "Платформа имеет 5 000 курсов, но пользователи не могут найти нужный контент. Конверсия в завершение курса — 12%, хотим поднять до 25%.",
				DataAndMaterials:   "JSON-фид с метаданными курсов (теги, уровень, продолжительность). Анонимизированная история просмотров (user_id, course_id, progress %).",
				ExpectedResult:     "Прототип рекомендательного движка с REST API и простым UI.",
				SuccessCriteria:    "Precision@5 >= 0.4 на тестовой выборке; время отклика API < 200 мс.",
				Constraints:        "Без хранения персональных данных. Открытый стек. Деплой локально.",
				TargetUsers:        "Студенты и специалисты, проходящие онлайн-обучение.",
				ContactAndFeedback: "https://edu-platform.example.com/feedback",
			},
		},
		{
			ID: "task-5", Title: "Мониторинг качества воздуха в реальном времени", Topic: "Экология",
			Status: "open",
			Fields: domain.TaskFields{
				ContextAndNeed:     "В 5 районах города установлены датчики PM2.5/PM10/CO2, но данные не агрегируются. Жители не получают предупреждений при превышении норм ВОЗ.",
				DataAndMaterials:   "WebSocket-стрим с 20 датчиков (JSON, 1 раз/мин). Нормативы ВОЗ в CSV. Карта GeoJSON с расположением датчиков.",
				ExpectedResult:     "Веб-дашборд с картой, графиками и push-уведомлениями при превышении порогов.",
				SuccessCriteria:    "Задержка отображения < 5 с; 100% датчиков на карте; уведомление < 30 с после превышения нормы.",
				Constraints:        "Работать в браузере без установки. Мобильная адаптация желательна.",
				TargetUsers:        "Жители города и сотрудники городской экологической службы.",
				ContactAndFeedback: "eco@city.example.kz",
			},
		},
	}

	teams := []domain.Team{
		{ID: "team-1", Name: "Data Pioneers", Skills: []string{"Python", "SQL", "Machine Learning", "Pandas"}, Focus: "Анализ данных и ML"},
		{ID: "team-2", Name: "Code Crafters", Skills: []string{"JavaScript", "React", "Go", "UI/UX"}, Focus: "Веб-разработка"},
		{ID: "team-3", Name: "Green Minds", Skills: []string{"Python", "IoT", "Grafana", "MQTT"}, Focus: "Устойчивое развитие и экология"},
		{ID: "team-4", Name: "Route Masters", Skills: []string{"Python", "Алгоритмы оптимизации", "OR-Tools"}, Focus: "Логистика и маршрутизация"},
		{ID: "team-5", Name: "Edu Innovators", Skills: []string{"Go", "Дизайн", "Аналитика", "UX Research"}, Focus: "Образовательные технологии"},
	}

	proposals := []domain.Proposal{
		{ID: "prop-1", TaskID: "task-3", TeamID: "team-1", SolutionIdea: "Применим LSTM для предсказания аномального потребления. Дашборд на Plotly Dash с алертами по email.", Plan: "", Status: "PENDING"},
		{ID: "prop-2", TaskID: "task-3", TeamID: "team-3", SolutionIdea: "IoT-стрим через MQTT → InfluxDB → Grafana. Алерты через Telegram-бота.", Plan: "", Status: "PENDING"},
		{ID: "prop-3", TaskID: "task-4", TeamID: "team-5", SolutionIdea: "Collaborative Filtering + content-based гибрид. FastAPI + простой React-интерфейс.", Plan: "", Status: "PENDING"},
		{ID: "prop-4", TaskID: "task-5", TeamID: "team-2", SolutionIdea: "WebSocket-дашборд на React + Leaflet карта. Service Worker для push-уведомлений.", Plan: "", Status: "PENDING"},
		{ID: "prop-5", TaskID: "task-5", TeamID: "team-3", SolutionIdea: "Python + MQTT subscriber, Grafana + GeoMap panel, Alert Manager для SMS.", Plan: "", Status: "PENDING"},
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	rollback := func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			log.Printf("seed rollback: %v", rbErr)
		}
	}

	for i := range tasks {
		score, level, _ := rating.Calculate(tasks[i].Fields)
		tasks[i].Rating = score
		tasks[i].ReadinessLevel = level

		fieldsJSON, err := json.Marshal(tasks[i].Fields)
		if err != nil {
			rollback()
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO tasks (id, title, topic, fields, rating, readiness_level, status) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			tasks[i].ID, tasks[i].Title, tasks[i].Topic, string(fieldsJSON),
			tasks[i].Rating, tasks[i].ReadinessLevel, tasks[i].Status,
		); err != nil {
			rollback()
			return err
		}
	}

	for _, t := range teams {
		skillsJSON, err := json.Marshal(t.Skills)
		if err != nil {
			rollback()
			return err
		}
		if _, err := tx.Exec(`INSERT INTO teams (id, name, skills, focus) VALUES (?, ?, ?, ?)`,
			t.ID, t.Name, string(skillsJSON), t.Focus,
		); err != nil {
			rollback()
			return err
		}
	}

	for _, p := range proposals {
		if _, err := tx.Exec(
			`INSERT INTO proposals (id, task_id, team_id, solution_idea, plan, status) VALUES (?, ?, ?, ?, ?, ?)`,
			p.ID, p.TaskID, p.TeamID, p.SolutionIdea, p.Plan, p.Status,
		); err != nil {
			rollback()
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	log.Println("Seed data inserted.")
	return nil
}
