// Package handlers содержит HTTP обработчики для API endpoints.
// GetTaskHandler обрабатывает GET запросы на получение информации о задании.
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"at-api/models"
	"at-api/services"
)

// GetTaskHandler обрабатывает GET /api/v1/tasks/:id - получение задания по ID.
// Извлекает ID задания из URL пути и возвращает информацию о задании.
// Возвращает 404 если задание не найдено, 200 с данными задания при успехе.
func GetTaskHandler(taskService *services.TaskService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем ID из URL пути (предполагается формат /api/v1/tasks/{id})
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(pathParts) < 4 {
			respondWithError(w, http.StatusBadRequest, "Invalid URL format")
			return
		}

		// Парсим ID задания
		idStr := pathParts[3]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid task ID")
			return
		}

		// Получаем задание из сервиса
		task, err := taskService.GetTask(id)
		if err != nil {
			if err == services.ErrTaskNotFound {
				respondWithError(w, http.StatusNotFound, "Task not found")
				return
			}
			respondWithError(w, http.StatusInternalServerError, "Failed to get task")
			return
		}

		// Возвращаем задание
		respondWithJSON(w, http.StatusOK, models.TaskResponse{Task: task})
	}
}
