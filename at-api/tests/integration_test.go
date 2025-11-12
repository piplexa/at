//
// Так запускаем тест: go test integration_test.go -v
// Все тесты: go test

package tests

import (
	"log"
	"testing"

	"bytes"
	"net/http"

	"encoding/json"

	"time"
)

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

func TestAPI_With_Worker(t *testing.T) {
	var apiURL = "http://localhost:8080"

	// Креды для базы данных; TODO: вынести в файл
	var dbConfig = DBConfig{
		Host:     "postgres",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		Database: "at_scheduler",
	}

	// Настройка вывода логов
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// вызов API
	// time.Sleep() или ожидание
	// проверка БД

	log.Printf("apiURL: %s", apiURL)
	log.Printf("dbConfig: %#v", dbConfig)

	// Текущее время плюс 10 секунд
	var t_wait time.Duration = 10
	tnow := time.Now().Add(t_wait * time.Second)
	var tnow_string string = tnow.Format(time.RFC3339)
	log.Printf("time now: %s", tnow_string)

	// формируем тело запроса
	data := map[string]any{
		"execute_at": tnow_string,
		"task_type":  "http_callback",
		"payload": map[string]any{
			"url":    "http://at-api:8080/health", // вызов будет внутри контейнера at-worker! В сети где "живут" эти сервисы
			"method": "GET",
			"data": map[string]any{
				"param1": "value1",
			},
		},
	}

	// конвертим в json-строку
	res, _ := json.Marshal(data)
	log.Printf("data: %s", res)

	// Создадим задание
	res1, err := http.Post(
		apiURL+"/api/v1/tasks",
		"application/json",
		bytes.NewReader(res),
	)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// TODO: Через t_wait + пару секунд, надо сходить в базу и проверить результат работы worker, в error_message должен быть OK

	log.Printf("res: %v", res1)
}
