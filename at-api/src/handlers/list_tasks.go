// Package handlers содержит HTTP обработчики для API endpoints.
// ListTasksHandler обрабатывает GET запросы на получение списка заданий с фильтрацией.
package handlers

import (
	"net/http"
	"strconv"

	"at-api/models"
	"at-api/services"
)

// ListTasksHandler обрабатывает GET /api/v1/tasks - получение списка заданий.
// Поддерживает query параметры:
//   - status: фильтр по статусу (pending, processing, completed, failed, cancelled)
//   - task_type: фильтр по типу задания
//   - limit: количество записей на странице (по умолчанию 50, максимум 100)
//   - offset: смещение для пагинации (по умолчанию 0)
//
// Возвращает массив заданий и общее количество записей.
func ListTasksHandler(taskService *services.TaskService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Парсим query параметры
		query := r.URL.Query()

		// Параметры фильтрации
		params := models.ListTasksParams{
			Status:   query.Get("status"),
			TaskType: query.Get("task_type"),
		}

		// Парсим limit
		if limitStr := query.Get("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err != nil || limit < 0 {
				respondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
				return
			}
			params.Limit = limit
		}

		// Парсим offset
		if offsetStr := query.Get("offset"); offsetStr != "" {
			offset, err := strconv.Atoi(offsetStr)
			if err != nil || offset < 0 {
				respondWithError(w, http.StatusBadRequest, "Invalid offset parameter")
				return
			}
			params.Offset = offset
		}

		// Получаем список заданий
		tasks, total, err := taskService.ListTasks(params)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to list tasks")
			return
		}

		// Возвращаем результат
		respondWithJSON(w, http.StatusOK, models.TaskListResponse{
			Tasks: tasks,
			Total: total,
		})
	}
}
