// Package handlers содержит HTTP обработчики для API endpoints.
// CreateTaskHandler обрабатывает POST запросы на создание новых заданий.
package handlers

import (
	"encoding/json"
	"net/http"

	"at-api/models"
	"at-api/services"
)

// CreateTaskHandler обрабатывает POST /api/v1/tasks - создание нового задания.
// Принимает JSON с полями: execute_at, task_type, payload, max_attempts (опционально).
// Возвращает созданное задание со статусом 201 Created или ошибку.
func CreateTaskHandler(taskService *services.TaskService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Декодируем JSON из тела запроса
		var req models.CreateTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Валидация обязательных полей
		if req.ExecuteAt.IsZero() {
			respondWithError(w, http.StatusBadRequest, "execute_at is required")
			return
		}
		if req.TaskType == "" {
			respondWithError(w, http.StatusBadRequest, "task_type is required")
			return
		}
		if len(req.Payload) == 0 {
			respondWithError(w, http.StatusBadRequest, "payload is required")
			return
		}

		// Создаем задание через сервис
		task, err := taskService.CreateTask(&req)
		if err != nil {
			if err == services.ErrInvalidExecuteTime {
				respondWithError(w, http.StatusBadRequest, err.Error())
				return
			}
			respondWithError(w, http.StatusInternalServerError, "Failed to create task")
			return
		}

		// Возвращаем созданное задание
		respondWithJSON(w, http.StatusCreated, models.TaskResponse{Task: task})
	}
}

// respondWithJSON отправляет JSON ответ с указанным статус кодом.
// Используется для возврата успешных ответов с данными.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to marshal response"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// respondWithError отправляет JSON ответ с ошибкой.
// Используется для возврата ошибок клиенту в унифицированном формате.
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, models.ErrorResponse{Error: message})
}
