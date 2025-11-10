// Package models содержит модели данных для работы с запланированными заданиями.
// Определяет структуру ScheduledTask, соответствующую таблице scheduled_tasks в БД.
// Используется для чтения заданий из базы данных и обработки их в worker.
package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// ScheduledTask представляет запланированное задание в системе.
// Структура соответствует таблице scheduled_tasks в PostgreSQL.
// Содержит всю информацию о задании: время выполнения, тип, payload, статус и метаданные.
type ScheduledTask struct {
	ID           int64           `json:"id"`
	ExecuteAt    time.Time       `json:"execute_at"`
	TaskType     string          `json:"task_type"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	Attempts     int             `json:"attempts"`
	MaxAttempts  int             `json:"max_attempts"`
	ErrorMessage sql.NullString  `json:"error_message,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	CompletedAt  sql.NullTime    `json:"completed_at,omitempty"`
}

// TaskResult представляет результат выполнения задания.
// Содержит ID задания, признак успешности выполнения и сообщение об ошибке (если есть).
type TaskResult struct {
	TaskID       int64
	Success      bool
	ErrorMessage string
}
