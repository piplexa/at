// Package handlers содержит HTTP обработчики для API endpoints.
// CancelTaskHandler обрабатывает DELETE запросы на отмену задания.
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"at-api/models"
	"at-api/services"
)

// CancelTaskHandler обрабатывает DELETE /api/v1/tasks/:id - отмена задания.
// Устанавливает статус задания в 'cancelled'.
// Возвращает 404 если задание не найдено, 200 с обновленными данными при успехе.
// Можно отменить только задания в статусе 'pending' или 'processing'.
func CancelTaskHandler(taskService *services.TaskService) http.HandlerFunc {
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

		// Отменяем задание через сервис
		task, err := taskService.CancelTask(id)
		if err != nil {
			if err == services.ErrTaskNotFound {
				respondWithError(w, http.StatusNotFound, "Task not found or cannot be cancelled")
				return
			}
			respondWithError(w, http.StatusInternalServerError, "Failed to cancel task")
			return
		}

		// Возвращаем обновленное задание
		respondWithJSON(w, http.StatusOK, models.TaskResponse{Task: task})
	}
}
