// Package tests - интеграционные HTTP тесты для AT API.
// Тесты выполняют реальные HTTP запросы к запущенному API сервису.
// Перед запуском тестов необходимо:
// 1. Запустить PostgreSQL с примененными миграциями
// 2. Запустить at-api сервис
//
// Запуск: go test -v
// С custom URL: API_URL=http://localhost:9090 go test -v
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

var (
	// apiURL - базовый URL для API (можно переопределить через переменную окружения)
	apiURL = getEnv("API_URL", "http://localhost:8080")
)

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TaskResponse - структура ответа с заданием
type TaskResponse struct {
	Task *Task `json:"task"`
}

// TaskListResponse - структура ответа со списком заданий
type TaskListResponse struct {
	Tasks []Task `json:"tasks"`
	Total int    `json:"total"`
}

// Task - структура задания
type Task struct {
	ID           int64           `json:"id"`
	ExecuteAt    string          `json:"execute_at"`
	TaskType     string          `json:"task_type"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	Attempts     int             `json:"attempts"`
	MaxAttempts  int             `json:"max_attempts"`
	ErrorMessage interface{}     `json:"error_message"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
	CompletedAt  interface{}     `json:"completed_at"`
}

// ErrorResponse - структура ответа с ошибкой
type ErrorResponse struct {
	Error string `json:"error"`
}

// TestHealthCheck проверяет работу health check endpoint
func TestHealthCheck(t *testing.T) {
	t.Log("Testing GET /health")

	resp, err := http.Get(apiURL + "/health")
	if err != nil {
		t.Fatalf("Failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health check failed: status=%d, want=200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("Health check body: got=%s, want=OK", string(body))
	}

	t.Log("✅ Health check passed")
}

// TestCreateTask проверяет создание задания
func TestCreateTask(t *testing.T) {
	t.Log("Testing POST /api/v1/tasks")

	// Подготавливаем данные для создания задания
	futureTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	reqBody := map[string]interface{}{
		"execute_at":   futureTime,
		"task_type":    "test_task",
		"payload":      map[string]string{"test": "data", "key": "value"},
		"max_attempts": 3,
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(
		apiURL+"/api/v1/tasks",
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Create task failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var taskResp TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if taskResp.Task == nil {
		t.Fatal("Task is nil in response")
	}

	if taskResp.Task.ID == 0 {
		t.Error("Task ID should not be 0")
	}

	if taskResp.Task.TaskType != "test_task" {
		t.Errorf("TaskType: got=%s, want=test_task", taskResp.Task.TaskType)
	}

	if taskResp.Task.Status != "pending" {
		t.Errorf("Status: got=%s, want=pending", taskResp.Task.Status)
	}

	if taskResp.Task.MaxAttempts != 3 {
		t.Errorf("MaxAttempts: got=%d, want=3", taskResp.Task.MaxAttempts)
	}

	t.Logf("✅ Task created successfully with ID=%d", taskResp.Task.ID)
}

// TestCreateTaskInvalidData проверяет создание задания с невалидными данными
func TestCreateTaskInvalidData(t *testing.T) {
	t.Log("Testing POST /api/v1/tasks with invalid data")

	testCases := []struct {
		name string
		body map[string]interface{}
		want int
	}{
		{
			name: "missing execute_at",
			body: map[string]interface{}{
				"task_type": "test",
				"payload":   map[string]string{"key": "value"},
			},
			want: http.StatusBadRequest,
		},
		{
			name: "missing task_type",
			body: map[string]interface{}{
				"execute_at": time.Now().Add(1 * time.Hour).Format(time.RFC3339),
				"payload":    map[string]string{"key": "value"},
			},
			want: http.StatusBadRequest,
		},
		{
			name: "execute_at in past",
			body: map[string]interface{}{
				"execute_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				"task_type":  "test",
				"payload":    map[string]string{"key": "value"},
			},
			want: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tc.body)
			resp, err := http.Post(
				apiURL+"/api/v1/tasks",
				"application/json",
				bytes.NewReader(jsonData),
			)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.want {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Status: got=%d, want=%d, body=%s", resp.StatusCode, tc.want, string(body))
			} else {
				t.Logf("✅ Correctly rejected: %s", tc.name)
			}
		})
	}
}

// TestFullCycle проверяет полный цикл работы с заданием:
// создание -> получение -> отмена
func TestFullCycle(t *testing.T) {
	t.Log("Testing full cycle: create -> get -> cancel")

	// 1. Создаем задание
	futureTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	reqBody := map[string]interface{}{
		"execute_at":   futureTime,
		"task_type":    "full_cycle_test",
		"payload":      map[string]string{"test": "full_cycle"},
		"max_attempts": 5,
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := http.Post(
		apiURL+"/api/v1/tasks",
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Create failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var createResp TaskResponse
	json.NewDecoder(resp.Body).Decode(&createResp)
	taskID := createResp.Task.ID
	t.Logf("Created task ID=%d", taskID)

	// 2. Получаем задание по ID
	resp, err = http.Get(fmt.Sprintf("%s/api/v1/tasks/%d", apiURL, taskID))
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Get failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var getResp TaskResponse
	json.NewDecoder(resp.Body).Decode(&getResp)

	if getResp.Task.ID != taskID {
		t.Errorf("Task ID mismatch: got=%d, want=%d", getResp.Task.ID, taskID)
	}

	if getResp.Task.TaskType != "full_cycle_test" {
		t.Errorf("TaskType: got=%s, want=full_cycle_test", getResp.Task.TaskType)
	}

	if getResp.Task.Status != "pending" {
		t.Errorf("Status: got=%s, want=pending", getResp.Task.Status)
	}

	t.Logf("✅ Got task ID=%d, status=%s", getResp.Task.ID, getResp.Task.Status)

	// 3. Отменяем задание
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/tasks/%d", apiURL, taskID), nil)
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to cancel task: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Cancel failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var cancelResp TaskResponse
	json.NewDecoder(resp.Body).Decode(&cancelResp)

	if cancelResp.Task.Status != "cancelled" {
		t.Errorf("Status after cancel: got=%s, want=cancelled", cancelResp.Task.Status)
	}

	t.Logf("✅ Task cancelled successfully, status=%s", cancelResp.Task.Status)
}

// TestGetTaskNotFound проверяет получение несуществующего задания
func TestGetTaskNotFound(t *testing.T) {
	t.Log("Testing GET /api/v1/tasks/:id with non-existent ID")

	resp, err := http.Get(fmt.Sprintf("%s/api/v1/tasks/999999", apiURL))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Status: got=%d, want=404", resp.StatusCode)
	} else {
		t.Log("✅ Correctly returned 404 for non-existent task")
	}
}

// TestListTasks проверяет получение списка заданий
func TestListTasks(t *testing.T) {
	t.Log("Testing GET /api/v1/tasks")

	// Создаем несколько заданий для теста
	for i := 0; i < 3; i++ {
		futureTime := time.Now().Add(time.Duration(i+1) * time.Hour).Format(time.RFC3339)
		reqBody := map[string]interface{}{
			"execute_at":   futureTime,
			"task_type":    fmt.Sprintf("list_test_%d", i),
			"payload":      map[string]string{"index": fmt.Sprintf("%d", i)},
			"max_attempts": 3,
		}
		jsonData, _ := json.Marshal(reqBody)
		resp, _ := http.Post(apiURL+"/api/v1/tasks", "application/json", bytes.NewReader(jsonData))
		resp.Body.Close()
	}

	// Получаем список заданий
	resp, err := http.Get(apiURL + "/api/v1/tasks")
	if err != nil {
		t.Fatalf("Failed to get tasks list: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("List failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var listResp TaskListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(listResp.Tasks) == 0 {
		t.Error("Expected at least some tasks in the list")
	}

	if listResp.Total == 0 {
		t.Error("Expected total > 0")
	}

	t.Logf("✅ Got %d tasks, total=%d", len(listResp.Tasks), listResp.Total)
}

// TestListTasksWithFilters проверяет фильтрацию списка заданий
func TestListTasksWithFilters(t *testing.T) {
	t.Log("Testing GET /api/v1/tasks with filters")

	// Создаем задание с уникальным типом
	uniqueType := fmt.Sprintf("filter_test_%d", time.Now().Unix())
	futureTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	reqBody := map[string]interface{}{
		"execute_at":   futureTime,
		"task_type":    uniqueType,
		"payload":      map[string]string{"test": "filter"},
		"max_attempts": 3,
	}
	jsonData, _ := json.Marshal(reqBody)
	resp, _ := http.Post(apiURL+"/api/v1/tasks", "application/json", bytes.NewReader(jsonData))
	resp.Body.Close()

	// Фильтруем по task_type
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/tasks?task_type=%s", apiURL, uniqueType))
	if err != nil {
		t.Fatalf("Failed to get filtered tasks: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Filter failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var listResp TaskListResponse
	json.NewDecoder(resp.Body).Decode(&listResp)

	if len(listResp.Tasks) == 0 {
		t.Error("Expected at least one task with filter")
	}

	// Проверяем, что все задания имеют правильный тип
	for _, task := range listResp.Tasks {
		if task.TaskType != uniqueType {
			t.Errorf("Task type mismatch: got=%s, want=%s", task.TaskType, uniqueType)
		}
	}

	t.Logf("✅ Filter by task_type works, found %d tasks", len(listResp.Tasks))
}

// TestListTasksWithPagination проверяет пагинацию
func TestListTasksWithPagination(t *testing.T) {
	t.Log("Testing GET /api/v1/tasks with pagination")

	// Получаем первую страницу с limit=2
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/tasks?limit=2&offset=0", apiURL))
	if err != nil {
		t.Fatalf("Failed to get paginated tasks: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Pagination failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var listResp TaskListResponse
	json.NewDecoder(resp.Body).Decode(&listResp)

	if len(listResp.Tasks) > 2 {
		t.Errorf("Expected max 2 tasks, got %d", len(listResp.Tasks))
	}

	t.Logf("✅ Pagination works, got %d tasks (limit=2), total=%d", len(listResp.Tasks), listResp.Total)
}
