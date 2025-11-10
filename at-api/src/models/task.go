// Package models содержит модели данных для работы с запланированными заданиями.
// Определяет структуру ScheduledTask, соответствующую таблице scheduled_tasks в БД,
// а также структуры для запросов и ответов API.
package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// ScheduledTask представляет запланированное задание в системе.
// Структура соответствует таблице scheduled_tasks в PostgreSQL.
type ScheduledTask struct {
	ID          int64           `json:"id"`
	ExecuteAt   time.Time       `json:"execute_at"`
	TaskType    string          `json:"task_type"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	ErrorMessage sql.NullString `json:"error_message,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CompletedAt sql.NullTime    `json:"completed_at,omitempty"`
}

// CreateTaskRequest представляет запрос на создание нового задания.
// Используется в POST /api/v1/tasks
type CreateTaskRequest struct {
	ExecuteAt   time.Time       `json:"execute_at"`
	TaskType    string          `json:"task_type"`
	Payload     json.RawMessage `json:"payload"`
	MaxAttempts int             `json:"max_attempts,omitempty"`
}

// ListTasksParams содержит параметры для фильтрации списка заданий.
// Используется в GET /api/v1/tasks
type ListTasksParams struct {
	Status   string // Фильтр по статусу: pending, processing, completed, failed, cancelled
	TaskType string // Фильтр по типу задания
	Limit    int    // Количество записей на странице
	Offset   int    // Смещение для пагинации
}

// TaskResponse представляет успешный ответ с данными задания
type TaskResponse struct {
	Task *ScheduledTask `json:"task"`
}

// TaskListResponse представляет ответ со списком заданий
type TaskListResponse struct {
	Tasks []ScheduledTask `json:"tasks"`
	Total int             `json:"total"`
}

// ErrorResponse представляет ответ с ошибкой
type ErrorResponse struct {
	Error string `json:"error"`
}
