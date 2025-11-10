// Package main - точка входа для AT API сервера.
// Инициализирует подключение к БД, создает HTTP сервер с роутингом для всех endpoints.
// Сервер предоставляет REST API для управления запланированными заданиями.
package main

import (
	"fmt"
	"log"
	"net/http"

	"at-api/config"
	"at-api/db"
	"at-api/handlers"
	"at-api/services"

	"github.com/joho/godotenv"
)

func main() {
	// Пытаемся загрузить .env файл, если он существует
	// Если файла нет, используем переменные окружения системы
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	} else {
		log.Println("Loaded configuration from .env file")
	}

	// Загружаем конфигурацию из переменных окружения
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Подключаемся к базе данных
	database, err := db.NewPostgresDB(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	log.Println("Successfully connected to database")

	// Создаем сервис для работы с заданиями
	taskService := services.NewTaskService(database)

	// Настраиваем роутинг
	mux := http.NewServeMux()

	// API endpoints
	// Используем паттерн с "/" на конце, чтобы ловить все подпути
	mux.HandleFunc("/api/v1/tasks/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlers.CreateTaskHandler(taskService)(w, r)
		case http.MethodGet:
			// Проверяем, есть ли ID в пути
			if r.URL.Path != "/api/v1/tasks/" && r.URL.Path != "/api/v1/tasks" {
				handlers.GetTaskHandler(taskService)(w, r)
			} else {
				handlers.ListTasksHandler(taskService)(w, r)
			}
		case http.MethodDelete:
			handlers.CancelTaskHandler(taskService)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Запускаем сервер
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("Starting AT API server on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
