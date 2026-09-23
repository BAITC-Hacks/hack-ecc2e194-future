package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type seedTask struct {
	id             string
	title          string
	topic          string
	fields         []string
	rating         int
	readinessLevel string
	status         string
}

type seedTeam struct {
	id     string
	name   string
	skills []string
	focus  string
}

type TaskFields struct {
	ContextAndNeed     string `json:"context_and_need"`
	DataAndMaterials   string `json:"data_and_materials"`
	ExpectedResult     string `json:"expected_result"`
	SuccessCriteria    string `json:"success_criteria"`
	Constraints        string `json:"constraints"`
	TargetUsers        string `json:"target_users"`
	ContactAndFeedback string `json:"contact_and_feedback"`
}

type taskResponse struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Topic          string     `json:"topic"`
	Fields         TaskFields `json:"fields"`
	Rating         int        `json:"rating"`
	ReadinessLevel string     `json:"readiness_level"`
	Status         string     `json:"status"`
}

type analyzeDraftRequest struct {
	DraftText string `json:"draft_text"`
}

type draftQuestion struct {
	Field string `json:"field"`
	Text  string `json:"text"`
}

type analyzeDraftResponse struct {
	Topic     string          `json:"topic"`
	Questions []draftQuestion `json:"questions"`
}

type chatCompletionRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	ResponseFormat responseFormat `json:"response_format"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func analyzeDraftHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var input analyzeDraftRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	result := fallbackDraftAnalysis()
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey != "" {
		if analyzed, err := requestDraftAnalysis(r, apiKey, input.DraftText); err == nil {
			result = analyzed
		} else {
			log.Printf("OpenAI draft analysis failed; using fallback: %v", err)
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Printf("encode draft analysis response: %v", err)
	}
}

func fallbackDraftAnalysis() analyzeDraftResponse {
	return analyzeDraftResponse{
		Topic: "Не определена",
		Questions: []draftQuestion{
			{Field: "data_and_materials", Text: "Какие исходные данные и материалы будут предоставлены командам и в каком формате?"},
			{Field: "expected_result", Text: "Какой конкретный результат должна представить команда к завершению хакатона?"},
			{Field: "success_criteria", Text: "По каким измеримым метрикам будет оцениваться решение?"},
		},
	}
}

func requestDraftAnalysis(r *http.Request, apiKey, draft string) (analyzeDraftResponse, error) {
	payload := chatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []chatMessage{
			{Role: "system", Content: "Ты бизнес-аналитик. Выдели отрасль и задай ровно 3 конкретных уточняющих вопроса к черновику задачи, закрывающих пробелы по данным, ожидаемому результату и метрикам. Формат ответа строго JSON: {topic: string, questions: [{field: string, text: string}]}"},
			{Role: "user", Content: draft},
		},
		ResponseFormat: responseFormat{Type: "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return analyzeDraftResponse{}, err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return analyzeDraftResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return analyzeDraftResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return analyzeDraftResponse{}, fmt.Errorf("OpenAI API returned %s", resp.Status)
	}
	var completion chatCompletionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&completion); err != nil {
		return analyzeDraftResponse{}, err
	}
	if len(completion.Choices) == 0 {
		return analyzeDraftResponse{}, errors.New("OpenAI response has no choices")
	}
	var result analyzeDraftResponse
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &result); err != nil {
		return analyzeDraftResponse{}, err
	}
	if strings.TrimSpace(result.Topic) == "" || len(result.Questions) != 3 {
		return analyzeDraftResponse{}, errors.New("OpenAI response has invalid topic or question count")
	}
	for _, question := range result.Questions {
		if strings.TrimSpace(question.Field) == "" || strings.TrimSpace(question.Text) == "" {
			return analyzeDraftResponse{}, errors.New("OpenAI response contains an incomplete question")
		}
	}
	return result, nil
}

func CalculateRating(fields TaskFields) (int, string, []string) {
	rating := 0
	empty := make([]string, 0)
	checks := []struct {
		name      string
		value     string
		minLength int
		points    int
	}{
		{"context_and_need", fields.ContextAndNeed, 40, 20},
		{"data_and_materials", fields.DataAndMaterials, 20, 20},
		{"expected_result", fields.ExpectedResult, 25, 15},
		{"success_criteria", fields.SuccessCriteria, 20, 15},
		{"constraints", fields.Constraints, 15, 10},
		{"target_users", fields.TargetUsers, 10, 10},
	}
	for _, check := range checks {
		if len([]rune(strings.TrimSpace(check.value))) >= check.minLength {
			rating += check.points
		} else {
			empty = append(empty, check.name)
		}
	}
	if strings.Contains(fields.ContactAndFeedback, "@") || strings.Contains(fields.ContactAndFeedback, "http") {
		rating += 10
	} else {
		empty = append(empty, "contact_and_feedback")
	}

	level := "DRAFT"
	switch {
	case rating >= 90:
		level = "PRIORITY"
	case rating >= 70:
		level = "READY"
	case rating >= 40:
		level = "IN_PROGRESS"
	}
	return rating, level, empty
}

func main() {
	db, err := sql.Open("sqlite", "app.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	schema := []string{
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			title TEXT,
			topic TEXT,
			fields JSON,
			rating INTEGER,
			readiness_level TEXT,
			status TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS teams (
			id TEXT PRIMARY KEY,
			name TEXT,
			skills JSON,
			focus TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS proposals (
			id TEXT PRIMARY KEY,
			task_id TEXT,
			team_id TEXT,
			solution_idea TEXT,
			plan TEXT,
			status TEXT
		)`,
	}
	for _, statement := range schema {
		if _, err := db.Exec(statement); err != nil {
			log.Fatal(err)
		}
	}

	var taskCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&taskCount); err != nil {
		log.Fatal(err)
	}

	if taskCount == 0 {
		tasks := []seedTask{
			{"task-1", "Р СџРЎР‚Р С•Р С–Р Р…Р С•Р В· РЎРѓР С—РЎР‚Р С•РЎРѓР В°", "Р В Р С•Р В·Р Р…Р С‘РЎвЂЎР Р…Р В°РЎРЏ РЎвЂљР С•РЎР‚Р С–Р С•Р Р†Р В»РЎРЏ", []string{"data_and_materials", "expected_result", "success_criteria"}, 20, "Р Р…Р С‘Р В·Р С”Р С‘Р в„–", "open"},
			{"task-2", "Р С›Р С—РЎвЂљР С‘Р СР С‘Р В·Р В°РЎвЂ Р С‘РЎРЏ Р СР В°РЎР‚РЎв‚¬РЎР‚РЎС“РЎвЂљР С•Р Р†", "Р вЂєР С•Р С–Р С‘РЎРѓРЎвЂљР С‘Р С”Р В°", []string{"data_and_materials", "expected_result", "success_criteria"}, 35, "Р Р…Р С‘Р В·Р С”Р С‘Р в„–", "open"},
			{"task-3", "Р С’Р Р…Р В°Р В»Р С‘Р В· РЎРЊР Р…Р ВµРЎР‚Р С–Р С•Р С—Р С•РЎвЂљРЎР‚Р ВµР В±Р В»Р ВµР Р…Р С‘РЎРЏ", "Р В­Р Р…Р ВµРЎР‚Р С–Р ВµРЎвЂљР С‘Р С”Р В°", []string{"data_and_materials", "expected_result", "success_criteria"}, 55, "РЎРѓРЎР‚Р ВµР Т‘Р Р…Р С‘Р в„–", "open"},
			{"task-4", "Р РЋР С‘РЎРѓРЎвЂљР ВµР СР В° РЎР‚Р ВµР С”Р С•Р СР ВµР Р…Р Т‘Р В°РЎвЂ Р С‘Р в„–", "Р С›Р В±РЎР‚Р В°Р В·Р С•Р Р†Р В°Р Р…Р С‘Р Вµ", []string{"data_and_materials", "expected_result", "success_criteria"}, 75, "Р Р†РЎвЂ№РЎРѓР С•Р С”Р С‘Р в„–", "open"},
			{"task-5", "Р СљР С•Р Р…Р С‘РЎвЂљР С•РЎР‚Р С‘Р Р…Р С– Р С”Р В°РЎвЂЎР ВµРЎРѓРЎвЂљР Р†Р В° Р Р†Р С•Р В·Р Т‘РЎС“РЎвЂ¦Р В°", "Р В­Р С”Р С•Р В»Р С•Р С–Р С‘РЎРЏ", []string{"data_and_materials", "expected_result", "success_criteria"}, 95, "Р Р†РЎвЂ№РЎРѓР С•Р С”Р С‘Р в„–", "open"},
		}
		teams := []seedTeam{
			{"team-1", "Data Pioneers", []string{"Go", "Python", "Р В°Р Р…Р В°Р В»Р С‘РЎвЂљР С‘Р С”Р В° Р Т‘Р В°Р Р…Р Р…РЎвЂ№РЎвЂ¦"}, "Р СР В°РЎв‚¬Р С‘Р Р…Р Р…Р С•Р Вµ Р С•Р В±РЎС“РЎвЂЎР ВµР Р…Р С‘Р Вµ"},
			{"team-2", "Code Crafters", []string{"JavaScript", "Go", "UI/UX"}, "Р Р†Р ВµР В±-РЎР‚Р В°Р В·РЎР‚Р В°Р В±Р С•РЎвЂљР С”Р В°"},
			{"team-3", "Green Minds", []string{"Python", "IoT", "РЎРЊР С”Р С•Р В»Р С•Р С–Р С‘РЎРЏ"}, "РЎС“РЎРѓРЎвЂљР С•Р в„–РЎвЂЎР С‘Р Р†Р С•Р Вµ РЎР‚Р В°Р В·Р Р†Р С‘РЎвЂљР С‘Р Вµ"},
			{"team-4", "Route Masters", []string{"Р В°Р В»Р С–Р С•РЎР‚Р С‘РЎвЂљР СРЎвЂ№", "Python", "Р С•Р С—РЎвЂљР С‘Р СР С‘Р В·Р В°РЎвЂ Р С‘РЎРЏ"}, "Р В»Р С•Р С–Р С‘РЎРѓРЎвЂљР С‘Р С”Р В°"},
			{"team-5", "Edu Innovators", []string{"Go", "Р Т‘Р С‘Р В·Р В°Р в„–Р Р…", "Р В°Р Р…Р В°Р В»Р С‘РЎвЂљР С‘Р С”Р В°"}, "Р С•Р В±РЎР‚Р В°Р В·Р С•Р Р†Р В°РЎвЂљР ВµР В»РЎРЉР Р…РЎвЂ№Р Вµ РЎвЂљР ВµРЎвЂ¦Р Р…Р С•Р В»Р С•Р С–Р С‘Р С‘"},
		}

		tx, err := db.Begin()
		if err != nil {
			log.Fatal(err)
		}
		rollback := func() {
			if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
				log.Printf("rollback: %v", err)
			}
		}

		for _, t := range tasks {
			fields, err := json.Marshal(t.fields)
			if err != nil {
				rollback()
				log.Fatal(err)
			}
			if _, err := tx.Exec(
				`INSERT INTO tasks (id, title, topic, fields, rating, readiness_level, status)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				t.id, t.title, t.topic, string(fields), t.rating, t.readinessLevel, t.status,
			); err != nil {
				rollback()
				log.Fatal(err)
			}
		}

		for _, team := range teams {
			skills, err := json.Marshal(team.skills)
			if err != nil {
				rollback()
				log.Fatal(err)
			}
			if _, err := tx.Exec(
				`INSERT INTO teams (id, name, skills, focus) VALUES (?, ?, ?, ?)`,
				team.id, team.name, string(skills), team.focus,
			); err != nil {
				rollback()
				log.Fatal(err)
			}
		}

		if err := tx.Commit(); err != nil {
			log.Fatal(err)
		}
	}

	http.HandleFunc("/api/tasks/calculate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var fields TaskFields
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&fields); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		} else if err == nil {
			http.Error(w, "request must contain one JSON object", http.StatusBadRequest)
			return
		}
		rating, level, empty := CalculateRating(fields)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"rating": rating, "readiness_level": level, "empty_fields": empty,
		}); err != nil {
			log.Printf("encode calculation response: %v", err)
		}
	})

	http.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rows, err := db.Query(`SELECT id, title, topic, fields, rating, readiness_level, status
			FROM tasks WHERE lower(status) IN ('published', 'open')
			ORDER BY CASE upper(readiness_level)
				WHEN 'PRIORITY' THEN 0 WHEN 'READY' THEN 1 ELSE 2 END, rating DESC`)
		if err != nil {
			http.Error(w, "failed to load tasks", http.StatusInternalServerError)
			log.Printf("query tasks: %v", err)
			return
		}
		defer rows.Close()

		tasks := make([]taskResponse, 0)
		for rows.Next() {
			var task taskResponse
			var storedFields sql.NullString
			if err := rows.Scan(&task.ID, &task.Title, &task.Topic, &storedFields, &task.Rating, &task.ReadinessLevel, &task.Status); err != nil {
				http.Error(w, "failed to read tasks", http.StatusInternalServerError)
				log.Printf("scan task: %v", err)
				return
			}
			// Older seed rows stored a string array; expose it as an empty object
			// when it cannot be decoded into the current seven-field structure.
			if !storedFields.Valid || json.Unmarshal([]byte(storedFields.String), &task.Fields) != nil {
				task.Fields = TaskFields{}
			}
			tasks = append(tasks, task)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "failed to read tasks", http.StatusInternalServerError)
			log.Printf("iterate tasks: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(tasks); err != nil {
			log.Printf("encode tasks response: %v", err)
		}
	})

	http.HandleFunc("/api/ai/analyze-draft", analyzeDraftHandler)
	http.Handle("/", http.FileServer(http.Dir("./static")))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
