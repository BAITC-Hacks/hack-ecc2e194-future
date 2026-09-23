package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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

// ──────────────────────────────────────────
//  Domain types
// ──────────────────────────────────────────

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

type taskCreateInput struct {
	Title  string     `json:"title"`
	Topic  string     `json:"topic"`
	Fields TaskFields `json:"fields"`
	Status string     `json:"status"`
}

type teamResponse struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Skills []string `json:"skills"`
	Focus  string   `json:"focus"`
}

type proposalInput struct {
	TaskID       string `json:"task_id"`
	TeamID       string `json:"team_id"`
	SolutionIdea string `json:"solution_idea"`
	Plan         string `json:"plan"`
}

type proposalResponse struct {
	ID           string `json:"id"`
	TaskID       string `json:"task_id"`
	TeamID       string `json:"team_id"`
	TeamName     string `json:"team_name"`
	SolutionIdea string `json:"solution_idea"`
	Plan         string `json:"plan"`
	Status       string `json:"status"`
}

type proposalDecision struct {
	Status string `json:"status"`
}

// ──────────────────────────────────────────
//  Rating calculation
// ──────────────────────────────────────────

// CalculateRating returns (score 0-100, readiness level, list of missing field keys).
func CalculateRating(fields TaskFields) (int, string, []string) {
	rating := 0
	missing := make([]string, 0)
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
			missing = append(missing, check.name)
		}
	}
	if strings.Contains(fields.ContactAndFeedback, "@") || strings.Contains(fields.ContactAndFeedback, "http") {
		rating += 10
	} else {
		missing = append(missing, "contact_and_feedback")
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
	return rating, level, missing
}

// ──────────────────────────────────────────
//  Helpers
// ──────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode JSON response: %v", err)
	}
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ──────────────────────────────────────────
//  AI analyze-draft
// ──────────────────────────────────────────

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
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// heuristicDraftAnalysis inspects the draft text and returns contextual questions.
func heuristicDraftAnalysis(draft string) analyzeDraftResponse {
	lower := strings.ToLower(draft)

	topic := "Общий"
	switch {
	case containsAny(lower, "финанс", "банк", "кредит", "платёж", "оплат"):
		topic = "Финансы"
	case containsAny(lower, "логистик", "доставк", "маршрут", "склад", "груз"):
		topic = "Логистика"
	case containsAny(lower, "образован", "студент", "курс", "обучен", "школ"):
		topic = "Образование"
	case containsAny(lower, "медицин", "здоров", "пациент", "клиник", "врач"):
		topic = "Медицина"
	case containsAny(lower, "экологи", "энерги", "выброс", "зелен", "климат"):
		topic = "Экология"
	case containsAny(lower, "торговл", "магазин", "клиент", "продаж", "ритейл"):
		topic = "Ритейл"
	case containsAny(lower, "производств", "завод", "фабрик", "оборудован"):
		topic = "Производство"
	}

	// Choose the 3 most relevant questions based on missing information signals.
	questions := make([]draftQuestion, 0, 3)

	if !containsAny(lower, "данн", "dataset", "api", "таблиц", "файл", "база") {
		questions = append(questions, draftQuestion{
			Field: "data_and_materials",
			Text:  "Какие данные, файлы или API будут предоставлены команде для работы над задачей?",
		})
	}
	if !containsAny(lower, "результат", "прототип", "продукт", "решени", "приложен") {
		questions = append(questions, draftQuestion{
			Field: "expected_result",
			Text:  "Какой конкретный артефакт должна передать команда по итогам работы (прототип, отчёт, модель, демо)?",
		})
	}
	if !containsAny(lower, "метрик", "kpi", "критерий", "критери", "измер", "процент", "точност") {
		questions = append(questions, draftQuestion{
			Field: "success_criteria",
			Text:  "По каким измеримым критериям бизнес определит, что задача решена успешно?",
		})
	}
	if len(questions) < 3 && !containsAny(lower, "пользовател", "для кого", "аудитор", "клиент") {
		questions = append(questions, draftQuestion{
			Field: "target_users",
			Text:  "Кто является конечным пользователем решения и каковы его главные потребности?",
		})
	}
	if len(questions) < 3 && !containsAny(lower, "ограничен", "срок", "стек", "технолог", "бюджет") {
		questions = append(questions, draftQuestion{
			Field: "constraints",
			Text:  "Какие есть ограничения по технологиям, срокам или доступам для работы над задачей?",
		})
	}
	// Guarantee at least 3 questions
	defaults := []draftQuestion{
		{Field: "data_and_materials", Text: "Какие исходные данные и материалы будут предоставлены командам и в каком формате?"},
		{Field: "expected_result", Text: "Какой конкретный результат должна представить команда к завершению хакатона?"},
		{Field: "success_criteria", Text: "По каким измеримым метрикам будет оцениваться решение команды?"},
	}
	for _, d := range defaults {
		if len(questions) >= 3 {
			break
		}
		questions = append(questions, d)
	}

	return analyzeDraftResponse{Topic: topic, Questions: questions[:3]}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func requestOpenAIAnalysis(r *http.Request, apiKey, draft string) (analyzeDraftResponse, error) {
	payload := chatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []chatMessage{
			{Role: "system", Content: "Ты бизнес-аналитик. Выдели отрасль и задай ровно 3 конкретных уточняющих вопроса к черновику задачи, закрывающих пробелы по данным, ожидаемому результату и метрикам. Не добавляй факты которых нет в тексте. Формат ответа строго JSON: {\"topic\": string, \"questions\": [{\"field\": string, \"text\": string}]}"},
			{Role: "user", Content: draft},
		},
	}
	payload.ResponseFormat.Type = "json_object"
	body, err := json.Marshal(payload)
	if err != nil {
		return analyzeDraftResponse{}, err
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return analyzeDraftResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 7 * time.Second}).Do(req)
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
	for _, q := range result.Questions {
		if strings.TrimSpace(q.Field) == "" || strings.TrimSpace(q.Text) == "" {
			return analyzeDraftResponse{}, errors.New("OpenAI response contains an incomplete question")
		}
	}
	return result, nil
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
	draft := strings.TrimSpace(input.DraftText)
	if draft == "" {
		http.Error(w, "draft_text is required", http.StatusBadRequest)
		return
	}

	// Try OpenAI; fall back to heuristic analysis.
	result := heuristicDraftAnalysis(draft)
	if apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); apiKey != "" {
		if ai, err := requestOpenAIAnalysis(r, apiKey, draft); err == nil {
			result = ai
		} else {
			log.Printf("OpenAI analysis failed, using heuristic fallback: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// ──────────────────────────────────────────
//  HTTP Handlers
// ──────────────────────────────────────────

func registerHandlers(db *sql.DB) {
	// ── Rating calculation ─────────────────
	http.HandleFunc("/api/tasks/calculate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var fields TaskFields
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&fields); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		rating, level, empty := CalculateRating(fields)
		writeJSON(w, http.StatusOK, map[string]any{
			"rating": rating, "readiness_level": level, "empty_fields": empty,
		})
	})

	// ── Tasks list + create ────────────────
	http.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			tasksListHandler(db, w, r)
		case http.MethodPost:
			taskCreateHandler(db, w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// ── Task proposals (GET /api/tasks/:id/proposals) ──
	http.HandleFunc("/api/tasks/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// /api/tasks/{id}/proposals
		if len(parts) == 4 && parts[0] == "api" && parts[1] == "tasks" && parts[3] == "proposals" {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			taskProposalsHandler(db, w, r, parts[2])
			return
		}
		http.NotFound(w, r)
	})

	// ── Teams list ─────────────────────────
	http.HandleFunc("/api/teams", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rows, err := db.Query(`SELECT id, name, skills, focus FROM teams ORDER BY name`)
		if err != nil {
			log.Printf("query teams: %v", err)
			http.Error(w, "failed to load teams", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		teams := make([]teamResponse, 0)
		for rows.Next() {
			var t teamResponse
			var skillsJSON sql.NullString
			if err := rows.Scan(&t.ID, &t.Name, &skillsJSON, &t.Focus); err != nil {
				log.Printf("scan team: %v", err)
				http.Error(w, "failed to read teams", http.StatusInternalServerError)
				return
			}
			if skillsJSON.Valid {
				_ = json.Unmarshal([]byte(skillsJSON.String), &t.Skills)
			}
			if t.Skills == nil {
				t.Skills = []string{}
			}
			teams = append(teams, t)
		}
		if err := rows.Err(); err != nil {
			log.Printf("iterate teams: %v", err)
			http.Error(w, "failed to read teams", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, teams)
	})

	// ── Proposals create ───────────────────
	http.HandleFunc("/api/proposals", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var input proposalInput
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&input); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		input.TaskID = strings.TrimSpace(input.TaskID)
		input.TeamID = strings.TrimSpace(input.TeamID)
		input.SolutionIdea = strings.TrimSpace(input.SolutionIdea)
		if input.TaskID == "" || input.TeamID == "" || input.SolutionIdea == "" {
			http.Error(w, "task_id, team_id and solution_idea are required", http.StatusBadRequest)
			return
		}
		var exists int
		if err := db.QueryRow(`SELECT 1 FROM tasks WHERE id = ?`, input.TaskID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "task not found", http.StatusNotFound)
			} else {
				log.Printf("check task: %v", err)
				http.Error(w, "failed to validate task", http.StatusInternalServerError)
			}
			return
		}
		if err := db.QueryRow(`SELECT 1 FROM teams WHERE id = ?`, input.TeamID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "team not found", http.StatusNotFound)
			} else {
				log.Printf("check team: %v", err)
				http.Error(w, "failed to validate team", http.StatusInternalServerError)
			}
			return
		}
		id, err := newID()
		if err != nil {
			log.Printf("generate proposal id: %v", err)
			http.Error(w, "failed to create proposal", http.StatusInternalServerError)
			return
		}
		proposal := proposalResponse{
			ID: id, TaskID: input.TaskID, TeamID: input.TeamID,
			SolutionIdea: input.SolutionIdea, Plan: strings.TrimSpace(input.Plan), Status: "PENDING",
		}
		if _, err := db.Exec(
			`INSERT INTO proposals (id, task_id, team_id, solution_idea, plan, status) VALUES (?, ?, ?, ?, ?, ?)`,
			proposal.ID, proposal.TaskID, proposal.TeamID, proposal.SolutionIdea, proposal.Plan, proposal.Status,
		); err != nil {
			log.Printf("insert proposal: %v", err)
			http.Error(w, "failed to save proposal", http.StatusInternalServerError)
			return
		}
		_ = db.QueryRow(`SELECT name FROM teams WHERE id = ?`, proposal.TeamID).Scan(&proposal.TeamName)
		writeJSON(w, http.StatusCreated, proposal)
	})

	// ── Proposal decision ──────────────────
	http.HandleFunc("/api/proposals/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "proposals" || parts[3] != "decision" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var decision proposalDecision
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&decision); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if decision.Status != "ACCEPTED" && decision.Status != "REJECTED" {
			http.Error(w, `status must be "ACCEPTED" or "REJECTED"`, http.StatusBadRequest)
			return
		}
		result, err := db.Exec(`UPDATE proposals SET status = ? WHERE id = ?`, decision.Status, parts[2])
		if err != nil {
			log.Printf("update proposal decision: %v", err)
			http.Error(w, "failed to update proposal", http.StatusInternalServerError)
			return
		}
		updated, _ := result.RowsAffected()
		if updated == 0 {
			http.Error(w, "proposal not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": parts[2], "status": decision.Status})
	})

	// ── AI analyze-draft ───────────────────
	http.HandleFunc("/api/ai/analyze-draft", analyzeDraftHandler)

	// ── Static files ───────────────────────
	http.Handle("/", http.FileServer(http.Dir("./static")))
}

// ──────────────────────────────────────────
//  Route sub-handlers
// ──────────────────────────────────────────

func tasksListHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	topicFilter := strings.TrimSpace(q.Get("topic"))
	levelFilter := strings.ToUpper(strings.TrimSpace(q.Get("level")))

	query := `SELECT id, title, topic, fields, rating, readiness_level, status
		FROM tasks WHERE lower(status) IN ('published','open')`
	args := make([]any, 0)
	if topicFilter != "" {
		query += ` AND lower(topic) LIKE lower(?)`
		args = append(args, "%"+topicFilter+"%")
	}
	if levelFilter != "" {
		query += ` AND upper(readiness_level) = ?`
		args = append(args, levelFilter)
	}
	query += ` ORDER BY
		CASE upper(readiness_level)
			WHEN 'PRIORITY'    THEN 0
			WHEN 'READY'       THEN 1
			WHEN 'IN_PROGRESS' THEN 2
			ELSE 3
		END, rating DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("query tasks: %v", err)
		http.Error(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	tasks := make([]taskResponse, 0)
	for rows.Next() {
		var task taskResponse
		var storedFields sql.NullString
		if err := rows.Scan(&task.ID, &task.Title, &task.Topic, &storedFields, &task.Rating, &task.ReadinessLevel, &task.Status); err != nil {
			log.Printf("scan task: %v", err)
			http.Error(w, "failed to read tasks", http.StatusInternalServerError)
			return
		}
		if !storedFields.Valid || json.Unmarshal([]byte(storedFields.String), &task.Fields) != nil {
			task.Fields = TaskFields{}
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate tasks: %v", err)
		http.Error(w, "failed to read tasks", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func taskCreateHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var input taskCreateInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Topic = strings.TrimSpace(input.Topic)
	if input.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if input.Topic == "" {
		input.Topic = "Общее"
	}

	rating, level, _ := CalculateRating(input.Fields)

	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != "published" && status != "draft" {
		status = "published"
	}

	fieldsJSON, err := json.Marshal(input.Fields)
	if err != nil {
		log.Printf("marshal task fields: %v", err)
		http.Error(w, "failed to create task", http.StatusInternalServerError)
		return
	}

	id, err := newID()
	if err != nil {
		log.Printf("generate task id: %v", err)
		http.Error(w, "failed to create task", http.StatusInternalServerError)
		return
	}

	if _, err := db.Exec(
		`INSERT INTO tasks (id, title, topic, fields, rating, readiness_level, status) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, input.Title, input.Topic, string(fieldsJSON), rating, level, status,
	); err != nil {
		log.Printf("insert task: %v", err)
		http.Error(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, taskResponse{
		ID:             id,
		Title:          input.Title,
		Topic:          input.Topic,
		Fields:         input.Fields,
		Rating:         rating,
		ReadinessLevel: level,
		Status:         status,
	})
}

func taskProposalsHandler(db *sql.DB, w http.ResponseWriter, _ *http.Request, taskID string) {
	rows, err := db.Query(`
		SELECT p.id, p.task_id, p.team_id, COALESCE(t.name,''), p.solution_idea, p.plan, p.status
		FROM proposals p LEFT JOIN teams t ON t.id = p.team_id
		WHERE p.task_id = ? ORDER BY p.rowid DESC`, taskID)
	if err != nil {
		log.Printf("query task proposals: %v", err)
		http.Error(w, "failed to load proposals", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	proposals := make([]proposalResponse, 0)
	for rows.Next() {
		var p proposalResponse
		if err := rows.Scan(&p.ID, &p.TaskID, &p.TeamID, &p.TeamName, &p.SolutionIdea, &p.Plan, &p.Status); err != nil {
			log.Printf("scan proposal: %v", err)
			http.Error(w, "failed to read proposals", http.StatusInternalServerError)
			return
		}
		proposals = append(proposals, p)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate proposals: %v", err)
		http.Error(w, "failed to read proposals", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, proposals)
}

// ──────────────────────────────────────────
//  Seed data
// ──────────────────────────────────────────

func seedDatabase(db *sql.DB) {
	type seedTask struct {
		id     string
		title  string
		topic  string
		fields TaskFields
	}
	type seedTeam struct {
		id     string
		name   string
		skills []string
		focus  string
	}

	tasks := []seedTask{
		{
			id: "task-1", title: "Прогноз спроса на основе истории продаж", topic: "Ритейл",
			fields: TaskFields{
				ContextAndNeed:     "Компания теряет до 15% выручки из-за ошибочных заказов: часть товаров замораживается на складе, другие постоянно заканчиваются. Нам нужна система прогнозирования спроса.",
				DataAndMaterials:   "",
				ExpectedResult:     "",
				SuccessCriteria:    "",
				Constraints:        "",
				TargetUsers:        "",
				ContactAndFeedback: "",
			},
		},
		{
			id: "task-2", title: "Оптимизация маршрутов доставки последней мили", topic: "Логистика",
			fields: TaskFields{
				ContextAndNeed:     "Курьеры тратят до 40% времени в пробках и на неоптимальных маршрутах. Хотим сократить операционные расходы и время доставки.",
				DataAndMaterials:   "Имеется CSV с историей заказов за 12 месяцев (координаты, время доставки, вес посылки).",
				ExpectedResult:     "",
				SuccessCriteria:    "",
				Constraints:        "",
				TargetUsers:        "Диспетчеры и курьеры службы доставки.",
				ContactAndFeedback: "",
			},
		},
		{
			id: "task-3", title: "Анализ энергопотребления зданий", topic: "Экология",
			fields: TaskFields{
				ContextAndNeed:     "Управляем портфелем из 30 офисных зданий. Расходы на электроэнергию выросли на 22% за год, но мы не понимаем причин и не можем выявить аномалии потребления.",
				DataAndMaterials:   "API системы СКУД со счётчиками каждые 15 минут, данные метеостанции (температура, влажность). Исторические данные за 2 года.",
				ExpectedResult:     "Дашборд с аномалиями и рекомендациями по снижению потребления.",
				SuccessCriteria:    "Снижение выявленных аномалий на 30%, точность модели > 85%.",
				Constraints:        "Стек: Python или JS. Срок — 5 часов хакатона.",
				TargetUsers:        "Facility-менеджеры зданий.",
				ContactAndFeedback: "mailto:energy@example.com",
			},
		},
		{
			id: "task-4", title: "Система рекомендаций образовательных курсов", topic: "Образование",
			fields: TaskFields{
				ContextAndNeed:     "Платформа имеет 5 000 курсов, но пользователи не могут найти нужный контент. Конверсия в завершение курса — 12%, хотим поднять до 25%.",
				DataAndMaterials:   "JSON-фид с метаданными курсов (теги, уровень, продолжительность). Анонимизированная история просмотров (user_id, course_id, progress %).",
				ExpectedResult:     "Прототип рекомендательного движка с REST API и простым UI для демонстрации.",
				SuccessCriteria:    "Precision@5 >= 0.4 на тестовой выборке; время отклика API < 200 мс.",
				Constraints:        "Без хранения персональных данных. Открытый стек. Деплой локально.",
				TargetUsers:        "Студенты и специалисты, проходящие онлайн-обучение.",
				ContactAndFeedback: "https://edu-platform.example.com/feedback",
			},
		},
		{
			id: "task-5", title: "Мониторинг качества воздуха в реальном времени", topic: "Экология",
			fields: TaskFields{
				ContextAndNeed:     "В 5 районах города установлены датчики PM2.5/PM10/CO2, но данные не агрегируются. Жители не получают предупреждений при превышении норм ВОЗ.",
				DataAndMaterials:   "WebSocket-стрим с 20 датчиков (JSON, 1 раз/мин). Нормативы ВОЗ в CSV. Карта GeoJSON с расположением датчиков.",
				ExpectedResult:     "Веб-дашборд с картой, графиками и push-уведомлениями при превышении порогов.",
				SuccessCriteria:    "Задержка отображения данных < 5 с; 100% датчиков на карте; уведомление за < 30 с после превышения нормы.",
				Constraints:        "Работать в браузере без установки. Мобильная адаптация желательна, но не обязательна.",
				TargetUsers:        "Жители города и сотрудники городской экологической службы.",
				ContactAndFeedback: "eco@city.example.kz",
			},
		},
	}

	teams := []seedTeam{
		{"team-1", "Data Pioneers", []string{"Python", "SQL", "Machine Learning", "Pandas"}, "Анализ данных и ML"},
		{"team-2", "Code Crafters", []string{"JavaScript", "React", "Go", "UI/UX"}, "Веб-разработка"},
		{"team-3", "Green Minds", []string{"Python", "IoT", "Grafana", "MQTT"}, "Устойчивое развитие и экология"},
		{"team-4", "Route Masters", []string{"Python", "Алгоритмы оптимизации", "OR-Tools"}, "Логистика и маршрутизация"},
		{"team-5", "Edu Innovators", []string{"Go", "Дизайн", "Аналитика", "UX Research"}, "Образовательные технологии"},
	}

	type seedProposal struct {
		id, taskID, teamID, idea, plan string
	}
	proposals := []seedProposal{
		{"prop-1", "task-3", "team-1", "Применим LSTM для предсказания аномального потребления. Дашборд на Plotly Dash с алертами по email.", "https://github.com/example/energy-ml"},
		{"prop-2", "task-3", "team-3", "IoT-стрим через MQTT → InfluxDB → Grafana. Алерты через Telegram-бота.", ""},
		{"prop-3", "task-4", "team-5", "Collaborative Filtering + content-based гибрид. FastAPI + простой React-интерфейс.", "https://edu-rec.example.com"},
		{"prop-4", "task-5", "team-2", "WebSocket-дашборд на React + Leaflet карта. Service Worker для push-уведомлений.", "https://air-dashboard.example.com"},
		{"prop-5", "task-5", "team-3", "Python + MQTT subscriber, Grafana + GeoMap panel, Alert Manager для SMS.", ""},
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	rollback := func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("rollback: %v", err)
		}
	}

	for _, t := range tasks {
		rating, level, _ := CalculateRating(t.fields)
		fieldsJSON, _ := json.Marshal(t.fields)
		if _, err := tx.Exec(
			`INSERT INTO tasks (id, title, topic, fields, rating, readiness_level, status) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			t.id, t.title, t.topic, string(fieldsJSON), rating, level, "open",
		); err != nil {
			rollback()
			log.Fatal(err)
		}
	}
	for _, t := range teams {
		skillsJSON, _ := json.Marshal(t.skills)
		if _, err := tx.Exec(
			`INSERT INTO teams (id, name, skills, focus) VALUES (?, ?, ?, ?)`,
			t.id, t.name, string(skillsJSON), t.focus,
		); err != nil {
			rollback()
			log.Fatal(err)
		}
	}
	for _, p := range proposals {
		if _, err := tx.Exec(
			`INSERT INTO proposals (id, task_id, team_id, solution_idea, plan, status) VALUES (?, ?, ?, ?, ?, ?)`,
			p.id, p.taskID, p.teamID, p.idea, p.plan, "PENDING",
		); err != nil {
			rollback()
			log.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	log.Println("Seed data inserted.")
}

// ──────────────────────────────────────────
//  Main
// ──────────────────────────────────────────

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
			title TEXT NOT NULL DEFAULT '',
			topic TEXT NOT NULL DEFAULT '',
			fields JSON,
			rating INTEGER NOT NULL DEFAULT 0,
			readiness_level TEXT NOT NULL DEFAULT 'DRAFT',
			status TEXT NOT NULL DEFAULT 'published'
		)`,
		`CREATE TABLE IF NOT EXISTS teams (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			skills JSON,
			focus TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS proposals (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			team_id TEXT NOT NULL,
			solution_idea TEXT NOT NULL DEFAULT '',
			plan TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'PENDING'
		)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			log.Fatal(err)
		}
	}

	var taskCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&taskCount); err != nil {
		log.Fatal(err)
	}
	if taskCount == 0 {
		seedDatabase(db)
	}

	registerHandlers(db)
	log.Println("Server listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
